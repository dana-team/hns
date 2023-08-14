package controllers

import (
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	controllers "github.com/dana-team/hns/internals/controllers/subnamespace/defaults"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// sync computes the status of the subnamespace and enqueues other subnamespaces for reconciliation if needed
func (r *SubnamespaceReconciler) sync(snsParentNS, snsObject *utils.ObjectContext) (ctrl.Result, error) {
	ctx := snsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("syncing subnamespace")

	snsName := snsObject.Object.GetName()
	snsParentName := snsParentNS.Object.GetName()

	// compute the current resources allocated by the synced subnamesapce to its children
	// and the allocated still free to allocate
	snsChildren, err := utils.NewObjectContextList(snsObject.Ctx, snsObject.Client, &danav1.SubnamespaceList{}, client.InNamespace(snsName))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get children subnamespace objects under namespace '%s': "+err.Error(), snsName)
	}
	childrenRequests, resourceAllocatedToChildren := getResourcesAllocatedToSNSChildren(snsChildren)
	free := getFreeToAllocateSNSResources(snsObject, resourceAllocatedToChildren)
	if utils.GetSNSResourcePoolLabel(snsObject.Object) == "" {
		if err := setSNSResourcePoolLabel(snsParentNS, snsObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set ResourcePool label for subnamespace '%s': "+err.Error(), snsName)
		}
		logger.Info("successfully set ResourcePool label for subnamespace", "subnamespace", snsName)
		return ctrl.Result{Requeue: true}, nil
	}

	rqFlag, err := utils.IsRq(snsObject, danav1.SelfOffset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute isRq flag for subnamespace '%s': "+err.Error(), snsName)
	}

	isSNSResourcePool, err := utils.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace '%s' is a ResourcePool: "+err.Error(), snsName)
	}

	isParentNSResourcePool, err := utils.IsNamespaceResourcePool(snsParentNS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace '%s' is a ResourcePool: "+err.Error(), snsParentName)
	}

	isSNSUpperResourcePool, err := utils.IsSNSUpperResourcePool(snsObject)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace '%s' is an upper ResourcePool: "+err.Error(), snsName)
	}

	// if the subnamespace is a regular SNS (i.e. not a ResourcePool) OR it's an upper-rp, then create a corresponding
	// quota object for the subnamespace. The quota object can be either a ResourceQuota or a ClusterResourceQuota
	if !isSNSResourcePool || isSNSUpperResourcePool {
		if rqFlag {
			err := deleteLegacyCRQ(snsObject)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete legacy quota object for subnamespace '%s': "+err.Error(), snsName)
			}
		}

		if res, err := syncquotaObject(snsObject, rqFlag); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to sync quota object for subnamespace '%s': "+err.Error(), snsName)
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
			return ctrl.Result{}, fmt.Errorf("failed to delete quota object for subnamespace '%s': "+err.Error(), snsName)
		}
		logger.Info("successfully deleted quota object for subnamespace", "subnamespace", snsName)

		if err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec = corev1.ResourceQuotaSpec{}
			log = log.WithValues("updated subnamespace", "removed spec.resourcequotaspec")
			return object, log
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status for subnamespace '%s': "+err.Error(), snsName)
		}
	}

	snsParentNamespace := snsParentNS.Object.(*corev1.Namespace).Labels[danav1.Parent]
	parentSNS, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsParentName, Namespace: snsParentNamespace}, &danav1.Subnamespace{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object '%s': "+err.Error(), snsParentName)
	}

	if err := syncSNSAnnotations(snsObject, snsParentNS, parentSNS, rqFlag); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to sync annotations for subnamespace '%s': "+err.Error(), snsName)
	}
	logger.Info("successfully synced annotations for subnamespace", "subnamespace", snsName)

	if err := r.ensureSNSInDB(ctx, snsObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure presence in namespaceDB for subnamespace '%s': "+err.Error(), snsObject.Object.GetName())
	}
	logger.Info("successfully ensured presence in namespaceDB for subnamespace", "subnamespace", snsObject.Object.GetName())

	if IsUpdateNeeded(snsObject.Object, childrenRequests, resourceAllocatedToChildren, free) {
		if err := updateSNSResourcesStatus(snsObject, childrenRequests, resourceAllocatedToChildren, free); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set status for subnamespace '%s': "+err.Error(), snsName)
		}
	}
	logger.Info("successfully set status for subnamespace", "subnamespace", snsName)

	// trigger reconciliation for parent subnamespace so that it can be aware of
	// potential changes in one of its children
	r.enqueueSNSEvent(parentSNS.Object.GetName(), parentSNS.Object.GetNamespace())
	logger.Info("successfully enqueued parent subnamespace for reconcliation", "parent subnamespace", parentSNS.Object.GetName(), "subnamespace", snsName)

	if err := r.enqueueChildrenRPToSNSConversionEvents(snsObject, snsChildren); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to enqueue children subnamespaces of subnamespace '%s' for reconciliation: "+err.Error(), snsName)
	}

	// trigger the child subnamespaces if the subnamespace was converted into a ResourcePool
	// it will update the isUpperRp annotation and the quotas accordingly
	if isSNSResourcePool {
		if err := r.enqueueChildrenSNSToRPConversionEvents(snsObject, snsChildren); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to enqueue children subnamespaces of subnamespace '%s' for reconciliation: "+err.Error(), snsName)
		}
	}

	r.enqueueSNSNamespaceEvent(snsName)
	logger.Info("successfully enqueued namespace for reconcliation", "namespace", snsName, "subnamespace", snsName)

	return ctrl.Result{}, nil
}

// updateSNSResourcesStatus updates the resources-related fields of the status of a subnamespace object
func updateSNSResourcesStatus(snsObject *utils.ObjectContext, childrenRequests []danav1.Namespaces, allocated, free corev1.ResourceList) error {
	return snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Subnamespace).Status.Namespaces = childrenRequests
		object.(*danav1.Subnamespace).Status.Total.Allocated = allocated
		object.(*danav1.Subnamespace).Status.Total.Free = free
		return object, log
	})
}

// getResourcesAllocatedToSNSChildren returns a ResourceList which is the computation of the
// total quantity of resources allocated to the children of a subnamespace
func getResourcesAllocatedToSNSChildren(snsChildren *utils.ObjectContextList) ([]danav1.Namespaces, corev1.ResourceList) {
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
// looking at the total available resources in the subnamespace spec and the currently allocated resources
func getFreeToAllocateSNSResources(snsObject *utils.ObjectContext, allocated corev1.ResourceList) corev1.ResourceList {
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

// deleteLegacyCRQ deletes a pre-exising CRQ in cases where they should only be a RQ.
// This is needed because historically we used to only have CRQs and didn't introduce RQs until later
func deleteLegacyCRQ(snsObject *utils.ObjectContext) error {
	if utils.DoesSNSCrqExists(snsObject) {
		crqName := snsObject.Object.GetName()
		crqObj, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: crqName}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return err
		}
		if err := crqObj.EnsureDeleteObject(); err != nil {
			return err
		}
	}
	return nil
}

// syncSNSAnnotations syncs subnamespace annotations
func syncSNSAnnotations(snsObject, snsParentNS, parentSNS *utils.ObjectContext, isRq bool) error {
	annotations := snsObject.Object.GetAnnotations()

	if isRq {
		annotations[danav1.IsRq] = danav1.True
	} else {
		annotations[danav1.IsRq] = danav1.False
	}

	isUpperResourcePool, err := utils.IsSNSUpperResourcePool(snsObject)
	if err != nil {
		return err
	}

	if isUpperResourcePool {
		annotations[danav1.IsUpperRp] = danav1.True
		annotations[danav1.UpperRp] = snsObject.Object.GetName()
	} else {
		annotations[danav1.IsUpperRp] = danav1.False
		if upperRPName, err := GetUpperResourcePoolNameFromParent(snsObject, parentSNS); err != nil {
			return err
		} else {
			annotations[danav1.UpperRp] = upperRPName
		}
	}

	annotations[danav1.CrqPointer] = utils.GetCrqPointer(snsObject.Object)

	displayName := utils.GetNamespaceDisplayName(snsParentNS.Object) + "/" + snsObject.Object.GetName()
	annotations[danav1.OpenShiftDisplayName] = displayName
	annotations[danav1.DisplayName] = displayName

	if err := snsObject.AppendAnnotations(annotations); err != nil {
		return err
	}

	return nil
}

// GetUpperResourcePoolNameFromParent returns the name of the upper ResourcePool of a given subnamespace
// by looking at its parent
func GetUpperResourcePoolNameFromParent(sns *utils.ObjectContext, parentSNS *utils.ObjectContext) (string, error) {
	upperRPName := "none"
	snsParentName := parentSNS.GetName()

	isParentUpperResourcePool, err := utils.IsSNSUpperResourcePoolFromAnnotation(sns.Object)
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

// deleteNonUpperResourcePoolCRQ deletes the CRQ object corresponding to a non-upper ResourcePool
func deleteNonUpperResourcePoolCRQ(snsObject *utils.ObjectContext) error {
	quotaObject, err := utils.GetSNSQuotaObject(snsObject)
	if err != nil {
		return err
	}

	if err := quotaObject.EnsureDeleteObject(); err != nil {
		return err
	}

	return nil
}

// syncquotaObject syncs between the resources in the spec of the subnamespace and the quota object
// or creates the quota object if it does not exist
func syncquotaObject(snsObject *utils.ObjectContext, isRq bool) (ctrl.Result, error) {
	if exists, quotaObject, err := utils.DoesSNSQuotaObjectExist(snsObject); err != nil {
		return ctrl.Result{}, err
	} else if exists {
		resources := utils.GetSnsQuotaSpec(snsObject.Object).Hard
		return ctrl.Result{}, updateQuotaObjectHard(quotaObject, resources, isRq)
	}

	if res, err := ensureSNSQuotaObject(snsObject, isRq); err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

// enqueueSNSEvent enqueues subanmespace events in the SnsEvents channel to trigger SNS reconciliation
func (r *SubnamespaceReconciler) enqueueSNSEvent(snsName, snsNamespace string) {
	r.SNSEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snsName,
			Namespace: snsNamespace,
		},
	}}
}

// enqueueSNSNamespaceEvent enqueues namespace events in the ResourcePoolEvents channel to trigger NS reconciliation
func (r *SubnamespaceReconciler) enqueueSNSNamespaceEvent(snsName string) {
	r.NSEvents <- event.GenericEvent{Object: &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   snsName,
			Labels: map[string]string{danav1.Hns: "true"},
		},
	}}
}

// enqueueChildrenRPToSNSConversionEvents enqueues children subanmespace events
// in cases where the subnamespace was converted from ResourcePool to regular subnamespace
func (r *SubnamespaceReconciler) enqueueChildrenRPToSNSConversionEvents(snsObject *utils.ObjectContext, snsChildren *utils.ObjectContextList) error {
	snsName := snsObject.Object.GetName()

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		snsChild, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: sns.GetName(), Namespace: snsName}, &danav1.Subnamespace{})
		if err != nil {
			return err
		}

		isResourcePool, err := utils.IsChildUpperResourcePool(snsObject.Object, snsChild.Object)
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
// in cases where the subnamespace was converted from regular subnamespace to ResourcePool
func (r *SubnamespaceReconciler) enqueueChildrenSNSToRPConversionEvents(snsObject *utils.ObjectContext, snsChildren *utils.ObjectContextList) error {
	snsName := snsObject.Object.GetName()

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		snsChild, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: sns.GetName(), Namespace: snsObject.GetName()}, &danav1.Subnamespace{})
		if err != nil {
			return err
		}
		isUpperResourcePool := snsChild.Object.GetAnnotations()[danav1.IsUpperRp]
		if isUpperResourcePool == danav1.True {
			r.SNSEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snsChild.Object.GetName(),
					Namespace: snsName,
				},
			}}
		}
	}

	return nil
}

// IsUpdateNeeded gets a subnamespace object, a []danav1.Namespaces and two resource lists and returns whether
// the subnamespace object status has to be updated
func IsUpdateNeeded(subspace client.Object, childrenRequests []danav1.Namespaces, allocated, free corev1.ResourceList) bool {
	if !NamespacesEqual(subspace.(*danav1.Subnamespace).Status.Namespaces, childrenRequests) ||
		!ResourceListEqual(subspace.(*danav1.Subnamespace).Status.Total.Allocated, allocated) ||
		!ResourceListEqual(subspace.(*danav1.Subnamespace).Status.Total.Free, free) {
		return true
	}
	return false
}

// NamespacesEqual gets two []danav1.Namespaces and returns whether they are equal
func NamespacesEqual(nsA, nsB []danav1.Namespaces) bool {
	if len(nsA) != len(nsB) {
		return false
	}
	for i, nameQuotaPair := range nsA {
		if !ResourceQuotaSpecEqual(nameQuotaPair.ResourceQuotaSpec, nsB[i].ResourceQuotaSpec) {
			return false
		}
	}
	return true
}

// ResourceListEqual gets two ResourceLists and returns whether their specs are equal
func ResourceListEqual(resourceListA, resourceListB corev1.ResourceList) bool {
	if len(resourceListA) != len(resourceListB) {
		return false
	}

	for key, value1 := range resourceListA {
		value2, found := resourceListB[key]
		if !found {
			return false
		}
		if value1.Cmp(value2) != 0 {
			return false
		}
	}

	return true
}

// ResourceQuotaSpecEqual gets two ResourceQuotaSpecs and returns whether their specs are equal
func ResourceQuotaSpecEqual(resourceQuotaSpecA, resourceQuotaSpecB corev1.ResourceQuotaSpec) bool {
	var resources []string

	for resourceName := range controllers.ZeroedQuota.Hard {
		resources = append(resources, resourceName.String())
	}

	resourceQuotaSpecAFiltered := filterResources(resourceQuotaSpecA.Hard, resources)
	resourceQuotaSpecBFiltered := filterResources(resourceQuotaSpecB.Hard, resources)

	return ResourceListEqual(resourceQuotaSpecAFiltered, resourceQuotaSpecBFiltered)
}

// filterResources filters the given resourceList and returns a new resourceList
// that contains only the resources specified in the controlled resources list.
func filterResources(resourcesList corev1.ResourceList, resources []string) corev1.ResourceList {
	filteredList := corev1.ResourceList{}

	for resourceName, quantity := range resourcesList {
		if slices.Contains(resources, resourceName.String()) {
			addResourcesToList(&filteredList, quantity, resourceName.String())
		}
	}
	return filteredList
}

// addResourcesToList adds the given quantity of a resource with the specified name to the resource list.
// If the resource with the same name already exists in the list, it adds the quantity to the existing resource.
func addResourcesToList(resourcesList *corev1.ResourceList, quantity resource.Quantity, name string) {
	for resourceName, resourceQuantity := range *resourcesList {
		if name == string(resourceName) {
			resourceQuantity.Add(quantity)
			(*resourcesList)[resourceName] = resourceQuantity
			return
		}
	}
	(*resourcesList)[corev1.ResourceName(name)] = quantity
}
