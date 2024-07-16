package migrationhierarchy

import (
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/updatequota/upqutils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// increaseRootResources increases the resources of the root namespace quota object.
func increaseRootResources(mhObject *objectcontext.ObjectContext, rootNSName string, sourceResources corev1.ResourceQuotaSpec) error {
	rootNSQuotaObj, err := quota.RootNSObjectFromName(mhObject, rootNSName)
	if err != nil {
		return fmt.Errorf("failed to get root namespace quota object: %v", err.Error())
	}

	if err := addRootQuota(rootNSQuotaObj, sourceResources); err != nil {
		return fmt.Errorf("failed to add resources to root namespace quota object: %v", err.Error())
	}

	return nil

}

// decreaseRootResources decreases the resources of the root namespace quota object.
func decreaseRootResources(mhObject *objectcontext.ObjectContext, rootNSName string, sourceResources corev1.ResourceQuotaSpec) error {
	rootNSQuotaObj, err := quota.RootNSObjectFromName(mhObject, rootNSName)
	if err != nil {
		return fmt.Errorf("failed to get root namespace quota object: %v", err.Error())
	}

	if err := subRootQuota(rootNSQuotaObj, sourceResources); err != nil {
		return fmt.Errorf("failed to subtract resources from root namespace quota object: %v", err.Error())
	}

	return nil

}

// addRootQuota updates the resource quota for a root namespace by adding the quota specified
// in quotaSpec to the existing quota.
func addRootQuota(rootNSQuotaObj *objectcontext.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
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
func subRootQuota(rootNSQuotaObj *objectcontext.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := rootNSQuotaObj.EnsureUpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger, error) {
		for resourceName := range quotaSpec.Hard {
			var (
				before, _  = object.(*corev1.ResourceQuota).Spec.Hard[resourceName]
				request, _ = quotaSpec.Hard[resourceName]
			)
			before.Set(before.Value() - request.Value())
			object.(*corev1.ResourceQuota).Spec.Hard[resourceName] = before
		}
		return object, l, nil
	}, false)

	return err
}

// createMigrationUPQ performs an UpdateQuota with the resources given.
func createMigrationUPQ(mhObject *objectcontext.ObjectContext, sourceResources corev1.ResourceQuotaSpec, sourceNS, destNS string) error {
	mhName := mhObject.Name()

	if err := createUpdateQuota(mhObject, mhName, sourceNS, destNS, sourceResources); err != nil {
		return fmt.Errorf("failed to create updateQuota for migration %q: %v", mhObject.Name(), err.Error())
	}

	return nil
}

// monitorMigrationUPQ requeues the MigrationHierarchy object until the UpdateQuota is in Complete Phase,
// it checks the status of the UpdateQuota object created to provide resources for the migration.
func monitorMigrationUPQ(mhObject *objectcontext.ObjectContext, ns string) (ctrl.Result, error) {
	upqPhase, err := getUpdateQuotaStatus(mhObject, ns)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed getting updateQuota object status %q: %v", mhObject.Name(), err.Error())
	}

	if upqPhase == danav1.Error {
		return ctrl.Result{}, fmt.Errorf("failed to do updateQuota for migration %q, phase Error", mhObject.Name())
	}

	if upqPhase != danav1.Complete {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// createUpdateQuota creates an UpdateQuota object.
func createUpdateQuota(mhObject *objectcontext.ObjectContext, upqName, sourceNS, destNS string, resources corev1.ResourceQuotaSpec) error {
	description := "Automatically created by migration. Migration name: " + mhObject.Name()
	upq := upqutils.Compose(upqName, sourceNS, destNS, description, resources)

	upqObject, err := objectcontext.New(mhObject.Ctx, mhObject.Client, types.NamespacedName{}, upq)
	if err != nil {
		return err
	}

	return upqObject.CreateObject()
}

// getUpdateQuotaStatus requests an UpdateQuota object from the API Server and returns its phase.
func getUpdateQuotaStatus(mhObject *objectcontext.ObjectContext, upqNamespace string) (danav1.Phase, error) {
	upqName := mhObject.Name()
	upqObj, err := objectcontext.New(mhObject.Ctx, mhObject.Client, client.ObjectKey{Name: upqName, Namespace: upqNamespace}, &danav1.Updatequota{})

	if err != nil {
		return "", fmt.Errorf("failed getting updatequota object %q: %v", upqName, err.Error())
	}

	return upqObj.Object.(*danav1.Updatequota).Status.Phase, nil
}
