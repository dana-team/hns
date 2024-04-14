package subnamespace

import (
	"fmt"

	"github.com/dana-team/hns/internal/metrics"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/namespacedb"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// sync computes the status of the subnamespace and enqueues other subnamespaces for reconciliation if needed.
func (r *SubnamespaceReconciler) sync(snsParentNS, snsObject *objectcontext.ObjectContext) (ctrl.Result, error) {
	ctx := snsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("syncing subnamespace")

	snsName := snsObject.Name()
	snsParentName := snsParentNS.Name()

	// compute the current resources allocated by the synced subnamesapce to its children
	// and the allocated still free to allocate
	snsChildren, err := objectcontext.NewList(snsObject.Ctx, snsObject.Client, &danav1.SubnamespaceList{}, client.InNamespace(snsName))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get children subnamespace objects under namespace %q: "+err.Error(), snsName)
	}
	childrenRequests, resourceAllocatedToChildren := getResourcesAllocatedToSNSChildren(snsChildren)
	free := getFreeToAllocateSNSResources(snsObject, resourceAllocatedToChildren)
	if resourcepool.SNSLabel(snsObject.Object) == "" {
		if err := resourcepool.SetSNSResourcePoolLabel(snsParentNS, snsObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set ResourcePool label for subnamespace %q: "+err.Error(), snsName)
		}
		logger.Info("successfully set ResourcePool label for subnamespace", "subnamespace", snsName)
		return ctrl.Result{Requeue: true}, nil
	}

	rqFlag, err := quota.IsRQ(snsObject, danav1.SelfOffset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute isRq flag for subnamespace %q: "+err.Error(), snsName)
	}

	isSNSResourcePool, err := resourcepool.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace %q is a ResourcePool: "+err.Error(), snsName)
	}

	isParentNSResourcePool, err := resourcepool.IsNSResourcePool(snsParentNS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace %q is a ResourcePool: "+err.Error(), snsParentName)
	}

	isSNSUpperResourcePool, err := resourcepool.IsSNSUpper(snsObject)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace %q is an upper ResourcePool: "+err.Error(), snsName)
	}

	// if the subnamespace is a regular SNS (i.e. not a ResourcePool) OR it's an upper-rp, then create a corresponding
	// quota object for the subnamespace. The quota object can be either a ResourceQuota or a ClusterResourceQuota
	if !isSNSResourcePool || isSNSUpperResourcePool {
		exists, quotaObject, err := quota.DoesSNSCRQExists(snsObject)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get quota object")
		} else if exists && rqFlag {
			if err := quotaObject.EnsureDelete(); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get quota object")
			}
		}

		exists, quotaObject, err = quota.DoesSNSRQExists(snsObject)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get quota object")
		} else if exists && !rqFlag {
			if err := quotaObject.EnsureDelete(); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get quota object")
			}
		}

		if res, err := syncQuotaObject(snsObject, rqFlag); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to sync quota object for subnamespace %q: "+err.Error(), snsName)
		} else if !res.IsZero() {
			return res, nil
		}

	}
	logger.Info("successfully synced quota object for subnamespace", "subnamespace", snsName)

	// if the subnamespace and its parent are both ResourcePools then, if exists, delete the CRQ corresponding
	// to the synced SNS. This is needed in cases such as converting a Subnamespace with children to a ResourcePool:
	// in this case all the children of the converted subnamespace would turn into a ResourcePool as well and their
	// CRQ would have to be deleted, and the Spec would have to be updated appropriately
	if isSNSResourcePool && isParentNSResourcePool {
		err := deleteNonUpperResourcePoolCRQ(snsObject)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete quota object for subnamespace %q: "+err.Error(), snsName)
		}
		logger.Info("successfully deleted quota object for subnamespace", "subnamespace", snsName)

		if err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec = corev1.ResourceQuotaSpec{}
			log = log.WithValues("updated subnamespace", "removed spec.resourcequotaspec")
			return object, log
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status for subnamespace %q: "+err.Error(), snsName)
		}
	}

	snsParentNamespace := snsParentNS.Object.(*corev1.Namespace).Labels[danav1.Parent]
	parentSNS, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsParentName, Namespace: snsParentNamespace}, &danav1.Subnamespace{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object %q: "+err.Error(), snsParentName)
	}

	if err := syncSNSAnnotations(snsObject, snsParentNS, parentSNS, rqFlag); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to sync annotations for subnamespace %q: "+err.Error(), snsName)
	}
	logger.Info("successfully synced annotations for subnamespace", "subnamespace", snsName)

	if err := namespacedb.EnsureSNSInDB(ctx, snsObject, r.NamespaceDB); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure presence in namespacedb for subnamespace %q: "+err.Error(), snsObject.Name())
	}
	logger.Info("successfully ensured presence in namespacedb for subnamespace", "subnamespace", snsObject.Name())

	if IsUpdateNeeded(snsObject.Object, childrenRequests, resourceAllocatedToChildren, free) {
		if err := updateSNSResourcesStatus(snsObject, childrenRequests, resourceAllocatedToChildren, free); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set status for subnamespace %q: "+err.Error(), snsName)
		}
	}
	logger.Info("successfully set status for subnamespace", "subnamespace", snsName)

	updateSNSMetrics(snsName, snsParentName, free, resourceAllocatedToChildren, snsObject.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard)
	logger.Info("successfully set metrics for subnamespace", "subnamespace", snsName)

	// trigger reconciliation for parent subnamespace so that it can be aware of
	// potential changes in one of its children
	r.enqueueSNSEvent(parentSNS.Name(), parentSNS.Object.GetNamespace())
	logger.Info("successfully enqueued parent subnamespace for reconcliation", "parent subnamespace", parentSNS.Name(), "subnamespace", snsName)

	if err := r.enqueueChildrenRPToSNSConversionEvents(snsObject, snsChildren); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to enqueue children subnamespaces of subnamespace %q for reconciliation: "+err.Error(), snsName)
	}

	// trigger the child subnamespaces if the subnamespace was converted into a ResourcePool
	// it will update the isUpperRp annotation and the quotas accordingly
	if isSNSResourcePool {
		if err := r.enqueueChildrenSNSToRPConversionEvents(snsObject, snsChildren); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to enqueue children subnamespaces of subnamespace %q for reconciliation: "+err.Error(), snsName)
		}
	}

	r.enqueueSNSNamespaceEvent(snsName)
	logger.Info("successfully enqueued namespace for reconcliation", "namespace", snsName, "subnamespace", snsName)

	return ctrl.Result{}, nil
}

// updateSNSResourcesStatus updates the resources-related fields of the status of a subnamespace object.
func updateSNSResourcesStatus(snsObject *objectcontext.ObjectContext, childrenRequests []danav1.Namespaces, allocated, free corev1.ResourceList) error {
	return snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Subnamespace).Status.Namespaces = childrenRequests
		object.(*danav1.Subnamespace).Status.Total.Allocated = allocated
		object.(*danav1.Subnamespace).Status.Total.Free = free
		return object, log
	})
}

// getResourcesAllocatedToSNSChildren returns a ResourceList which is the computation of the
// total quantity of resources allocated to the children of a subnamespace.
func getResourcesAllocatedToSNSChildren(snsChildren *objectcontext.ObjectContextList) ([]danav1.Namespaces, corev1.ResourceList) {
	var childrenRequests []danav1.Namespaces
	var resourceAllocatedToChildren = corev1.ResourceList{}

	for _, childSNS := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		var childNameQuotaPair = danav1.Namespaces{
			Namespace:         childSNS.GetName(),
			ResourceQuotaSpec: childSNS.Spec.ResourceQuotaSpec,
		}
		childrenRequests = append(childrenRequests, childNameQuotaPair)

		childSNSRequest := childSNS.Spec.ResourceQuotaSpec.Hard
		for resourceName := range childSNS.Spec.ResourceQuotaSpec.Hard {
			var (
				totalRequest, _ = resourceAllocatedToChildren[resourceName]
				vRequest, _     = childSNSRequest[resourceName]
			)
			resourceAllocatedToChildren[resourceName] = *resource.NewQuantity(vRequest.Value()+totalRequest.Value(), resource.BinarySI)
		}
	}

	return childrenRequests, resourceAllocatedToChildren
}

// getFreeToAllocateSNSResources computes the resources that are still free to allocate by
// looking at the total available resources in the subnamespace spec and the currently allocated resources.
func getFreeToAllocateSNSResources(snsObject *objectcontext.ObjectContext, allocated corev1.ResourceList) corev1.ResourceList {
	var freeToAllocate = corev1.ResourceList{}

	for resourceName := range snsObject.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard {

		var (
			totalRequest, _ = allocated[resourceName]
			vRequest, _     = snsObject.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard[resourceName]
		)
		value := vRequest.Value() - totalRequest.Value()
		freeToAllocate[resourceName] = *resource.NewQuantity(value, resource.BinarySI)
	}

	return freeToAllocate
}

// syncSNSAnnotations syncs subnamespace annotations.
func syncSNSAnnotations(snsObject, snsParentNS, parentSNS *objectcontext.ObjectContext, isRq bool) error {
	annotations := snsObject.Object.GetAnnotations()

	if isRq {
		annotations[danav1.IsRq] = danav1.True
	} else {
		annotations[danav1.IsRq] = danav1.False
	}

	isUpperResourcePool, err := resourcepool.IsSNSUpper(snsObject)
	if err != nil {
		return err
	}

	if isUpperResourcePool {
		annotations[danav1.IsUpperRp] = danav1.True
		annotations[danav1.UpperRp] = snsObject.Name()
	} else {
		annotations[danav1.IsUpperRp] = danav1.False
		if upperRPName, err := GetUpperResourcePoolNameFromParent(snsObject, parentSNS); err != nil {
			return err
		} else {
			annotations[danav1.UpperRp] = upperRPName
		}
	}

	annotations[danav1.CrqPointer] = quota.GetCrqPointer(snsObject.Object)

	displayName := nsutils.DisplayName(snsParentNS.Object) + "/" + snsObject.Name()
	annotations[danav1.OpenShiftDisplayName] = displayName
	annotations[danav1.DisplayName] = displayName

	if err := snsObject.AppendAnnotations(annotations); err != nil {
		return err
	}

	return nil
}

// GetUpperResourcePoolNameFromParent returns the name of the upper ResourcePool of a given subnamespace
// by looking at its parent.
func GetUpperResourcePoolNameFromParent(sns *objectcontext.ObjectContext, parentSNS *objectcontext.ObjectContext) (string, error) {
	upperRPName := "none"
	snsParentName := parentSNS.Name()

	isParentUpperResourcePool, err := resourcepool.IsSNSUpperFromAnnotation(sns.Object)
	if err != nil {
		return "", err
	}

	if isParentUpperResourcePool {
		return snsParentName, nil
	}

	parentUpperRp, ok := parentSNS.Object.GetAnnotations()[danav1.UpperRp]
	if ok {
		upperRPName = parentUpperRp
	}

	return upperRPName, nil
}

// deleteNonUpperResourcePoolCRQ deletes the CRQ object corresponding to a non-upper ResourcePool.
func deleteNonUpperResourcePoolCRQ(snsObject *objectcontext.ObjectContext) error {
	quotaObject, err := quota.SubnamespaceObject(snsObject)
	if err != nil {
		return err
	}

	if err := quotaObject.EnsureDelete(); err != nil {
		return err
	}

	return nil
}

// syncQuotaObject syncs between the resources in the spec of the subnamespace and the quota object
// or creates the quota object if it does not exist.
func syncQuotaObject(snsObject *objectcontext.ObjectContext, isRq bool) (ctrl.Result, error) {
	if exists, quotaObject, err := quota.DoesSubnamespaceObjectExist(snsObject); err != nil {
		return ctrl.Result{}, err
	} else if exists {
		if err := quota.CreateDefaultSNSResourceQuota(snsObject); err != nil {
			return ctrl.Result{}, err
		}

		resources := quota.SubnamespaceSpec(snsObject.Object)
		return ctrl.Result{}, quota.UpdateObject(quotaObject, resources, isRq)
	}

	if res, err := quota.EnsureSubnamespaceObject(snsObject, isRq); err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

// enqueueSNSEvent enqueues subanmespace events in the SnsEvents channel to trigger SNS reconciliation.
func (r *SubnamespaceReconciler) enqueueSNSEvent(snsName, snsNamespace string) {
	r.SNSEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snsName,
			Namespace: snsNamespace,
		},
	}}
}

// enqueueSNSNamespaceEvent enqueues namespace events in the ResourcePoolEvents channel to trigger NS reconciliation.
func (r *SubnamespaceReconciler) enqueueSNSNamespaceEvent(snsName string) {
	r.NSEvents <- event.GenericEvent{Object: &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   snsName,
			Labels: map[string]string{danav1.Hns: "true"},
		},
	}}
}

// enqueueChildrenRPToSNSConversionEvents enqueues children subanmespace events
// in cases where the subnamespace was converted from ResourcePool to regular subnamespace.
func (r *SubnamespaceReconciler) enqueueChildrenRPToSNSConversionEvents(snsObject *objectcontext.ObjectContext, snsChildren *objectcontext.ObjectContextList) error {
	snsName := snsObject.Name()

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		snsChild, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: sns.GetName(), Namespace: snsName}, &danav1.Subnamespace{})
		if err != nil {
			return err
		}

		isResourcePool, err := resourcepool.IsChildUpper(snsObject.Object, snsChild.Object)
		if err != nil {
			return err
		}

		if isResourcePool {
			r.enqueueSNSEvent(sns.GetName(), snsName)

			r.SNSEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sns.GetName(),
					Namespace: snsName,
				},
			}}
		}
	}
	return nil
}

// enqueueChildrenSNSToRPConversionEvents enqueues children subanmespace events
// in cases where the subnamespace was converted from regular subnamespace to ResourcePool.
func (r *SubnamespaceReconciler) enqueueChildrenSNSToRPConversionEvents(snsObject *objectcontext.ObjectContext, snsChildren *objectcontext.ObjectContextList) error {
	snsName := snsObject.Name()

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		snsChild, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: sns.GetName(), Namespace: snsObject.Name()}, &danav1.Subnamespace{})
		if err != nil {
			return err
		}
		isUpperResourcePool := snsChild.Object.GetAnnotations()[danav1.IsUpperRp]
		if isUpperResourcePool == danav1.True {
			r.SNSEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snsChild.Name(),
					Namespace: snsName,
				},
			}}
		}
	}

	return nil
}

// IsUpdateNeeded gets a subnamespace object, a []danav1.Namespaces and two resource lists and returns whether
// the subnamespace object status has to be updated.
func IsUpdateNeeded(sns client.Object, childrenRequests []danav1.Namespaces, allocated, free corev1.ResourceList) bool {
	if !NamespacesEqual(sns.(*danav1.Subnamespace).Status.Namespaces, childrenRequests) ||
		!quota.ResourceListEqual(sns.(*danav1.Subnamespace).Status.Total.Allocated, allocated) ||
		!quota.ResourceListEqual(sns.(*danav1.Subnamespace).Status.Total.Free, free) {
		return true
	}
	return false
}

// NamespacesEqual gets two []danav1.Namespaces and returns whether they are equal.
func NamespacesEqual(nsA, nsB []danav1.Namespaces) bool {
	if len(nsA) != len(nsB) {
		return false
	}
	for i, nameQuotaPair := range nsA {
		if !quota.ResourceQuotaSpecEqual(nameQuotaPair.ResourceQuotaSpec, nsB[i].ResourceQuotaSpec) {
			return false
		}
	}
	return true
}

// updateSNSMetrics updates the metrics for the subnamespace.
func updateSNSMetrics(snsName, snsNS string, allocated, free, total corev1.ResourceList) {
	for resourceName := range total {
		allocatedResource := allocated[resourceName]
		freeResource := free[resourceName]
		totalResource := total[resourceName]

		metrics.ObserveSNSAllocatedResource(snsName, snsNS, resourceName.String(), allocatedResource.AsApproximateFloat64())
		metrics.ObserveSNSFreeResource(snsName, snsNS, resourceName.String(), freeResource.AsApproximateFloat64())
		metrics.ObserveSNSTotalResource(snsName, snsNS, resourceName.String(), totalResource.AsApproximateFloat64())
	}
}
