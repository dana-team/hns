package controllers

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
