package controllers

import (
	"context"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/controllers/subnamespace/defaults"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// setSNSResourcePoolLabel sets a ResourcePool label on the subnamespace based
// on the ResourcePool label in its parent namespace
func setSNSResourcePoolLabel(snsParentNS, snsObject *utils.ObjectContext) error {
	isResourcePool, err := utils.IsNamespaceResourcePool(snsParentNS)
	if err != nil {
		return err
	}

	if err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues(danav1.ResourcePool, isResourcePool)
		object.SetLabels(map[string]string{danav1.ResourcePool: strconv.FormatBool(isResourcePool)})
		return object, log
	}); err != nil {
		return err
	}

	return nil

}

// ensureSNSQuotaObject ensures that a quota object exists for a subnamespace
func ensureSNSQuotaObject(snsObject *utils.ObjectContext, isRq bool) (ctrl.Result, error) {
	quotaObjectName := snsObject.Object.GetName()
	quota := utils.GetSnsQuotaSpec(snsObject.Object)

	// if the subnamespace does not have a quota in its Spec, then set it to be equal to what it
	// currently uses, and create a quotaObject for it. This can happen when converting a ResourcePool to an SNS
	if len(quota.Hard) == 0 {
		quotaObject, err := setupQuotaObject(quotaObjectName, isRq, controllers.ZeroedQuota, snsObject)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := quotaObject.EnsureCreateObject(); err != nil {
			return ctrl.Result{}, err
		}

		// get the current used value from the quota object, so that we can later use the
		// value from its status to update the values in the spec of the quota object
		quotaObjectUsed, err := getQuotaObjectUsed(snsObject)
		if err != nil {
			return ctrl.Result{}, err
		}

		// if the value is nil then it means that it hasn't been created yet, so requeue
		if quotaObjectUsed == nil {
			return ctrl.Result{Requeue: true}, nil
		}

		if err := updateSnsQuotaSpec(snsObject, quotaObjectUsed); err != nil {
			return ctrl.Result{}, err
		}

		quota = corev1.ResourceQuotaSpec{Hard: quotaObjectUsed}
	}

	quotaObject, err := setupQuotaObject(quotaObjectName, isRq, quota, snsObject)
	if err != nil {
		return ctrl.Result{}, err
	}

	if utils.IsQuotaObjectZeroed(quotaObject.Object) {
		if err := updateQuotaObjectHard(quotaObject, quota.Hard, isRq); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := quotaObject.EnsureCreateObject(); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// setupQuotaObject sets up - but doesn't create - a quota object with the given resources, the quotaObject can be either
// a ResourceQuota or a ClusterResourceQuota based on the depth of the subnamespace
func setupQuotaObject(quotaObjName string, isRq bool, resources corev1.ResourceQuotaSpec, snsObject *utils.ObjectContext) (*utils.ObjectContext, error) {
	var quotaObj *utils.ObjectContext
	var err error

	if isRq {
		composedQuotaObj := composeRq(quotaObjName, quotaObjName, resources)
		quotaObj, err = utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: quotaObjName, Namespace: quotaObjName}, composedQuotaObj)
		if err != nil {
			return quotaObj, err
		}
	} else {
		selector, err := utils.GetSNSDepth(snsObject)
		if err != nil {
			return quotaObj, err
		}
		crqMap := map[string]string{danav1.CrqSelector + "-" + selector: quotaObjName}
		composedQuotaObj := composeCrq(quotaObjName, resources, crqMap)
		quotaObj, err = utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: quotaObjName}, composedQuotaObj)
		if err != nil {
			return quotaObj, err
		}
	}

	return quotaObj, nil
}

// composeRq returns a ResourceQuota object based on the given parameters
func composeRq(name, namespace string, quota corev1.ResourceQuotaSpec) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"rq.subnamespace": name},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: quota.Hard,
		},
	}
}

// composeCrq returns a ClusterResourceQuota object based on the given parameters
func composeCrq(name string, quota corev1.ResourceQuotaSpec, annSelector map[string]string) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"crq.subnamespace": name},
		},
		Spec: quotav1.ClusterResourceQuotaSpec{
			Selector: quotav1.ClusterResourceQuotaSelector{
				AnnotationSelector: annSelector,
			},
			Quota: quota,
		},
	}
}

// getQuotaObjectUsed returns the Used value of the quota object
func getQuotaObjectUsed(snsObject *utils.ObjectContext) (corev1.ResourceList, error) {
	quotaObj, err := utils.GetSNSQuotaObject(snsObject)
	if err != nil {
		return corev1.ResourceList{}, err
	}

	return utils.GetQuotaUsed(quotaObj.Object), nil
}

// updateQuotaObjectHard updates the spec of the quotaObject to be same as the given resources
func updateQuotaObjectHard(quotaObject *utils.ObjectContext, resources corev1.ResourceList, isRq bool) error {
	if isRq {
		return quotaObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			log = log.WithValues("updated subnamespace", "ResourceQuotaSpecHard", "resources", resources)
			object.(*corev1.ResourceQuota).Spec.Hard = resources
			return object, log
		})
	} else {
		return quotaObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			log = log.WithValues("updated subnamespace", "ResourceQuotaSpecQuotaHard", "resources", resources)
			object.(*quotav1.ClusterResourceQuota).Spec.Quota.Hard = resources
			return object, log
		})
	}
}

// ensureSubspaceInDB ensures subnamespace in db if it should be
func (r *SubnamespaceReconciler) ensureSNSInDB(ctx context.Context, sns *utils.ObjectContext) error {
	key := r.NamespaceDB.GetKey(sns.Object.GetName())
	if key != "" {
		return nil
	}

	rqFlag, err := utils.IsRq(sns, danav1.SelfOffset)
	if err != nil {
		return err
	}

	if !rqFlag {
		if err := namespaceDB.AddNS(ctx, r.NamespaceDB, r.Client, sns.Object.(*danav1.Subnamespace)); err != nil {
			return err
		}
	}
	return nil
}

// updateSnsQuotaSpec updates the ResourceQuotaSpec of a subnamespace to be equal to resources
func updateSnsQuotaSpec(snsObject *utils.ObjectContext, resources corev1.ResourceList) error {
	return snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated subnamespace", "ResourceQuotaSpecHard", "resources", resources)
		object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard = resources
		return object, log
	})
}
