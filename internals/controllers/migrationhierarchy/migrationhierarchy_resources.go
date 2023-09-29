package controllers

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// increaseRootResources increases the resources of the root namespace quota object
func increaseRootResources(mhObject *utils.ObjectContext, rootNSName string, sourceResources corev1.ResourceQuotaSpec) error {
	rootNSQuotaObj, err := utils.GetRootNSQuotaObjectFromName(mhObject, rootNSName)
	if err != nil {
		return fmt.Errorf("failed to get root namespace quota object: " + err.Error())
	}

	if err := addRootQuota(rootNSQuotaObj, sourceResources); err != nil {
		return fmt.Errorf("failed to add resources to root namespace quota object" + err.Error())
	}

	return nil

}

// decreaseRootResources decreases the resources of the root namespace quota object
func decreaseRootResources(mhObject *utils.ObjectContext, rootNSName string, sourceResources corev1.ResourceQuotaSpec) error {
	rootNSQuotaObj, err := utils.GetRootNSQuotaObjectFromName(mhObject, rootNSName)
	if err != nil {
		return fmt.Errorf("failed to get root namespace quota object: " + err.Error())
	}

	if err := subRootQuota(rootNSQuotaObj, sourceResources); err != nil {
		return fmt.Errorf("failed to subtract resources from root namespace quota object" + err.Error())
	}

	return nil

}

// addRootQuota updates the resource quota for a root namespace by adding the quota specified
// in quotaSpec to the existing quota.
func addRootQuota(rootNSQuotaObj *utils.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := rootNSQuotaObj.EnsureUpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger, error) {
		for resourceName := range quotaSpec.Hard {
			var (
				before, _  = object.(*corev1.ResourceQuota).Spec.Hard[resourceName]
				request, _ = quotaSpec.Hard[resourceName]
			)
			before.Set(before.Value() + request.Value())
			object.(*corev1.ResourceQuota).Spec.Hard[resourceName] = before
		}
		return object, l, nil
	}, false)

	return err
}

// subRootQuota updates the resource quota for a root namespace by subtracting the quota specified
// in quotaSpec to the existing quota.
func subRootQuota(rootNSQuotaObj *utils.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := rootNSQuotaObj.EnsureUpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger, error) {
		for resourceName := range quotaSpec.Hard {
			var (
				vBefore, _  = object.(*corev1.ResourceQuota).Spec.Hard[resourceName]
				vRequest, _ = quotaSpec.Hard[resourceName]
			)
			vBefore.Set(vBefore.Value() - vRequest.Value())
			object.(*corev1.ResourceQuota).Spec.Hard[resourceName] = vBefore
		}
		return object, l, nil
	}, false)

	return err
}

// createMigrationUPQ performs an UpdateQuota with the resources given
func createMigrationUPQ(mhObject *utils.ObjectContext, sourceResources corev1.ResourceQuotaSpec, sourceNS, destNS string) error {
	mhName := mhObject.Object.GetName()

	if err := createUpdateQuota(mhObject, mhName, sourceNS, destNS, sourceResources); err != nil {
		return fmt.Errorf("failed to create updateQuota for migration '%s'", mhObject.GetName())
	}

	return nil
}

// monitorMigrationUPQ requeues the MigrationHierarchy object until the UpdateQuota is in Complete Phase,
// it checks the status of the UpdateQuota object created to provide resources for the migration
func monitorMigrationUPQ(mhObject *utils.ObjectContext, ns string) (ctrl.Result, error) {
	upqPhase, err := getUpdateQuotaStatus(mhObject, ns)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed getting updateQuota object status '%s': "+err.Error(), mhObject.GetName())
	}

	if upqPhase == danav1.Error {
		return ctrl.Result{}, fmt.Errorf("failed to do updateQuota for migration '%s', phase Error", mhObject.GetName())
	}

	if upqPhase != danav1.Complete {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// createUpdateQuota creates an UpdateQuota object
func createUpdateQuota(mhObject *utils.ObjectContext, upqName, sourceNS, destNS string, resources corev1.ResourceQuotaSpec) error {
	description := "Automatically created by migration. Migration name: " + mhObject.Object.GetName()
	upq := utils.ComposeUpdateQuota(upqName, sourceNS, destNS, description, resources)

	upqObject, err := utils.NewObjectContext(mhObject.Ctx, mhObject.Client, types.NamespacedName{}, upq)
	if err != nil {
		return err
	}

	return upqObject.CreateObject()
}

// getUpdateQuotaStatus requests an UpdateQuota object from the API Server and returns its phase
func getUpdateQuotaStatus(mhObject *utils.ObjectContext, upqNamespace string) (danav1.Phase, error) {
	upqName := mhObject.Object.GetName()
	upqObj, err := utils.NewObjectContext(mhObject.Ctx, mhObject.Client, client.ObjectKey{Name: upqName, Namespace: upqNamespace}, &danav1.Updatequota{})

	if err != nil {
		return "", fmt.Errorf("failed getting updatequota object '%s': "+err.Error(), upqName)
	}

	return upqObj.Object.(*danav1.Updatequota).Status.Phase, nil
}
