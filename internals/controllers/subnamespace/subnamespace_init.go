package controllers

import (
	"fmt"
	defaults "github.com/dana-team/hns/internals/controllers/subnamespace/defaults"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

// init creates a namespace for the subnamespace, creates a quota object for it if needed, creates
// default ResourceQuota and LimitRange object in the namespace, and sets annotations on the subnamespace
func (r *SubnamespaceReconciler) init(snsParentNS, snsObject *utils.ObjectContext) (ctrl.Result, error) {
	ctx := snsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("initializing subnamespace")

	snsName := snsObject.Object.GetName()

	if err := createSNSNamespace(snsParentNS, snsObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create namespace for subnamespace '%s': "+err.Error(), snsName)
	}
	logger.Info("successfully created namespace for subnamespace", "subnamespace", snsName)

	snsResourcePoolLabel := utils.GetSNSResourcePoolLabel(snsObject.Object)
	if snsResourcePoolLabel == "" {
		if err := setSNSResourcePoolLabel(snsParentNS, snsObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set ResourcePool label for subnamespace '%s': "+err.Error(), snsName)
		}
	}
	logger.Info("successfully set ResourcePool label for subnamespace", "subnamespace", snsName)

	rqFlag, err := utils.IsRq(snsObject, danav1.SelfOffset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute isRq flag for subnamespace '%s': "+err.Error(), snsName)
	}

	// if the subnamespace is a regular SNS (i.e. not a ResourcePool) OR it's an upper-rp, then create a corresponding
	// quota object for the subnamespace. The quota object can be either a ResourceQuota or a ClusterResourceQuota
	isSNSResourcePool, err := utils.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace '%s' is a ResourcePool: "+err.Error(), snsName)
	}

	isSNSUpperResourcePool, err := utils.IsSNSUpperResourcePool(snsObject)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute if subnamespace '%s' is an upper ResourcePool: "+err.Error(), snsName)
	}

	if !isSNSResourcePool || isSNSUpperResourcePool {
		if res, err := ensureSNSQuotaObject(snsObject, rqFlag); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create quota object for subnamespace '%s': "+err.Error(), snsName)
		} else if !res.IsZero() {
			// requeue the reconciled object if needed
			return res, nil
		}
		logger.Info("successfully created quota object for subnamespace", "subnamespace", snsName)
	} else {
		if err := createDefaultSNSResourceQuota(snsObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create default ResourceQuota object for subnamespace '%s': "+err.Error(), snsName)
		}
		logger.Info("successfully created default ResourceQuota object for subnamespace", "subnamespace", snsName)
	}

	if err := createDefaultSNSLimitRange(snsObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create default Limit Range object for subnamespace '%s': "+err.Error(), snsName)
	}
	logger.Info("successfully created default LimitRange object for subnamespace", "subnamespace", snsName)

	annotations := make(map[string]string)
	displayName := utils.GetNamespaceDisplayName(snsParentNS.Object) + "/" + snsObject.Object.GetName()
	annotations[danav1.OpenShiftDisplayName] = displayName
	annotations[danav1.DisplayName] = displayName
	if rqFlag {
		annotations[danav1.IsRq] = danav1.True
	} else {
		annotations[danav1.IsRq] = danav1.False
	}

	if err := snsObject.AppendAnnotations(annotations); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to append for subnamespace '%s': "+err.Error(), snsName)
	}
	logger.Info("successfully appended annotations for subnamespace", "subnamespace", snsObject.Object.GetName())

	if err := r.ensureSNSInDB(ctx, snsObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure presence in namespaceDB for subnamespace '%s'", snsObject.Object.GetName())
	}
	logger.Info("successfully ensured presence in namespaceDB for subnamespace", "subnamespace", snsObject.Object.GetName())

	if err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Subnamespace).Status.Phase = danav1.Created
		log = log.WithValues("updated subnamespace phase", danav1.Created)
		return object, log
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set status '%s' for subnamespace '%s'", danav1.Created, snsName)
	}
	logger.Info("successfully set status for subnamespace", "phase", danav1.Created, "subnamespace", snsName)

	return ctrl.Result{}, nil
}

// createSNSNamespace makes sure a namespace is created for subnamespace
func createSNSNamespace(snsParentNS, snsObject *utils.ObjectContext) error {
	snsName := snsObject.Object.GetName()
	snsNamespace := composeSNSNamespace(snsParentNS, snsObject)

	snsNamespaceObject, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsName}, snsNamespace)
	if err != nil {
		return err
	}

	if snsNamespaceObject.IsPresent() {
		if err := syncNSLabelsAnnotations(snsNamespace, snsNamespaceObject); err != nil {
			return err
		}
	}

	if err := snsNamespaceObject.EnsureCreateObject(); err != nil {
		return err
	}

	return nil
}

// syncNSLabelsAnnotations copies the labels and annotations from a namespace
// to the objectContext representing it
func syncNSLabelsAnnotations(snsNamespace *corev1.Namespace, snsNamespaceObject *utils.ObjectContext) error {
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
// and annotations based on the namespace linked to the parent of the subnamespace
func composeSNSNamespace(snsParentNS, snsObject *utils.ObjectContext) *corev1.Namespace {
	nsName := snsObject.Object.GetName()
	labels, annotations := utils.GetNSLabelsAnnotationsBasedOnParent(snsParentNS, nsName)

	// add the ResourcePool label separately from the function
	labels[danav1.ResourcePool] = snsObject.Object.GetLabels()[danav1.ResourcePool]

	return ComposeNamespace(nsName, labels, annotations)
}

// createDefaultSNSResourceQuota creates a ResourceQuota object with some default values
// that we would like to limit and are not set by the user. This is only created in subnamespaces that
// have a ClusterResourceQuota
func createDefaultSNSResourceQuota(snsObject *utils.ObjectContext) error {
	snsName := snsObject.Object.GetName()
	composedDefaultRQ := ComposeResourceQuota(snsName, snsName, defaults.DefaultQuotaHard)

	snsDefaultRQ, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsName, Namespace: snsName}, composedDefaultRQ)
	if err != nil {
		return err
	}

	err = snsDefaultRQ.EnsureCreateObject()

	return err
}

func ComposeResourceQuota(name string, namespace string, hard corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		TypeMeta: metav1.TypeMeta{
			Kind: "ResourceQuota",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}
}

// createDefaultSNSLimitRange creates a limit range object with some default values
// that we would like to limit and are not set by the user
func createDefaultSNSLimitRange(snsObject *utils.ObjectContext) error {
	snsName := snsObject.Object.GetName()
	composedDefaultLimitRange := ComposeLimitRange(snsName, snsName, defaults.Limits)

	childLimitRange, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsName, Namespace: snsName}, composedDefaultLimitRange)
	if err != nil {
		return err
	}

	err = childLimitRange.EnsureCreateObject()

	return err
}

// ComposeLimitRange returns a LimitRange object based on the given parameters
func ComposeLimitRange(name string, namespace string, limits corev1.LimitRangeItem) *corev1.LimitRange {
	return &corev1.LimitRange{
		TypeMeta: metav1.TypeMeta{
			Kind: "LimitRange",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				limits,
			},
		},
	}
}

// ComposeNamespace returns a namespace object based on the given parameters
func ComposeNamespace(name string, labels map[string]string, annotations map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}
