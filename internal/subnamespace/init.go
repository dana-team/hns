package subnamespace

import (
	"fmt"

	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/namespacedb"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	danav1 "github.com/dana-team/hns/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// init creates a namespace for the subnamespace, creates a quota object for it if needed, creates
// default ResourceQuota and LimitRange object in the namespace, and sets  on the subnamespace.
func (r *SubnamespaceReconciler) init(snsParentNS, snsObject *objectcontext.ObjectContext) (ctrl.Result, error) {
	ctx := snsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("initializing subnamespace")

	snsName := snsObject.Name()

	if err := createSNSNamespace(snsParentNS, snsObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create namespace for subnamespace %q: "+err.Error(), snsName)
	}
	logger.Info("successfully created namespace for subnamespace", "subnamespace", snsName)

	snsResourcePoolLabel := resourcepool.SNSLabel(snsObject.Object)
	if snsResourcePoolLabel == "" {
		if err := resourcepool.SetSNSResourcePoolLabel(snsParentNS, snsObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set ResourcePool label for subnamespace %q: "+err.Error(), snsName)
		}
	}
	logger.Info("successfully set ResourcePool label for subnamespace", "subnamespace", snsName)

	rqFlag, err := quota.IsRQ(snsObject, danav1.SelfOffset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute isRq flag for subnamespace %q: "+err.Error(), snsName)
	}

	// if the subnamespace is a regular SNS (i.e. not a ResourcePool) OR it's an upper-rp, then create a corresponding
	// quota object for the subnamespace. The quota object can be either a ResourceQuota or a ClusterResourceQuota
	isSNSResourcePool, err := resourcepool.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace %q is a ResourcePool: "+err.Error(), snsName)
	}

	isSNSUpperResourcePool, err := resourcepool.IsSNSUpper(snsObject)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace %q is an upper ResourcePool: "+err.Error(), snsName)
	}

	if !isSNSResourcePool || isSNSUpperResourcePool {
		if res, err := quota.EnsureSubnamespaceObject(snsObject, rqFlag); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create quota object for subnamespace %q: "+err.Error(), snsName)
		} else if !res.IsZero() {
			// requeue the reconciled object if needed
			return res, nil
		}
		logger.Info("successfully created quota object for subnamespace", "subnamespace", snsName)
	} else {
		if err := quota.CreateDefaultSNSResourceQuota(snsObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create default ResourceQuota object for subnamespace %q: "+err.Error(), snsName)
		}
		logger.Info("successfully created default ResourceQuota object for subnamespace", "subnamespace", snsName)
	}

	if err := quota.CreateDefaultSNSLimitRange(snsObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create default Limit Range object for subnamespace %q: "+err.Error(), snsName)
	}
	logger.Info("successfully created default LimitRange object for subnamespace", "subnamespace", snsName)

	annotations := make(map[string]string)
	displayName := nsutils.DisplayName(snsParentNS.Object) + "/" + snsObject.Name()
	annotations[danav1.OpenShiftDisplayName] = displayName
	annotations[danav1.DisplayName] = displayName
	if rqFlag {
		annotations[danav1.IsRq] = danav1.True
	} else {
		annotations[danav1.IsRq] = danav1.False
	}

	if err := snsObject.AppendAnnotations(annotations); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to append for subnamespace %q: "+err.Error(), snsName)
	}
	logger.Info("successfully appended annotations for subnamespace", "subnamespace", snsObject.Name())

	if err := namespacedb.EnsureSNSInDB(ctx, snsObject, r.NamespaceDB); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure presence in namespacedb for subnamespace %q", snsObject.Name())
	}
	logger.Info("successfully ensured presence in namespacedb for subnamespace", "subnamespace", snsObject.Name())

	if err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Subnamespace).Status.Phase = danav1.Created
		log = log.WithValues("updated subnamespace phase", danav1.Created)
		return object, log
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set status %q for subnamespace %q", danav1.Created, snsName)
	}
	logger.Info("successfully set status for subnamespace", "phase", danav1.Created, "subnamespace", snsName)

	return ctrl.Result{}, nil
}

// createSNSNamespace makes sure a namespace is created for subnamespace.
func createSNSNamespace(snsParentNS, snsObject *objectcontext.ObjectContext) error {
	snsName := snsObject.Name()
	snsNamespace := composeSNSNamespace(snsParentNS, snsObject)

	snsNamespaceObject, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsName}, snsNamespace)
	if err != nil {
		return err
	}

	if snsNamespaceObject.IsPresent() {
		if err := syncNSLabelsAnnotations(snsNamespace, snsNamespaceObject); err != nil {
			return err
		}
	}

	if err := snsNamespaceObject.EnsureCreate(); err != nil {
		return err
	}

	return nil
}

// syncNSLabelsAnnotations copies the labels and annotations from a namespace
// to the objectContext representing it.
func syncNSLabelsAnnotations(snsNamespace *corev1.Namespace, snsNamespaceObject *objectcontext.ObjectContext) error {
	annotations := snsNamespace.GetAnnotations()
	if err := snsNamespaceObject.AppendAnnotations(annotations); err != nil {
		return err
	}

	labels := snsNamespace.GetLabels()
	if err := snsNamespaceObject.AppendLabels(labels); err != nil {
		return err
	}

	return nil
}

// composeSNSNamespace creates a new namespace for a particular subnamespace with labels
// and annotations based on the namespace linked to the parent of the subnamespace.
func composeSNSNamespace(snsParentNS, snsObject *objectcontext.ObjectContext) *corev1.Namespace {
	nsName := snsObject.Name()
	labels := nsutils.LabelsBasedOnParent(snsParentNS, nsName)
	annotations := nsutils.AnnotationsBasedOnParent(snsParentNS, nsName)

	// add the ResourcePool label separately from the function
	labels[danav1.ResourcePool] = snsObject.Object.GetLabels()[danav1.ResourcePool]

	return nsutils.ComposeNamespace(nsName, labels, annotations)
}
