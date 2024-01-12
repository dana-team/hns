package controllers

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// setup links the parent namespace with the subnamespace and changes the phase of
// the subnamespace to danav1.Missing
func (r *SubnamespaceReconciler) setup(snsParentNS, snsObject *utils.ObjectContext) (ctrl.Result, error) {
	ctx := snsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("setting up subnamespace")

	snsName := snsObject.Object.GetName()
	snsParentName := snsParentNS.Object.GetName()

	// sets the parent namespace as the owner of the subnamespace. This is used for garbage collection
	// of the owned object and for reconciling the owner object on changes to owned
	if err := ctrl.SetControllerReference(snsParentNS.Object, snsObject.Object, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("successfully set namespace as the owner reference of", "ownerReference", snsParentName, "subnamespace", snsName)

	err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated subnamespace phase", danav1.Missing, "namespaceRef", snsName)
		object.(*danav1.Subnamespace).Spec.NamespaceRef.Name = snsName
		object.(*danav1.Subnamespace).Status.Phase = danav1.Missing
		return object, log
	})

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set status '%s' for subnamespace '%s'", danav1.Missing, snsName)
	}

	logger.Info("successfully set status for subnamespace", "phase", danav1.Missing, "subnamespace", snsName)

	return ctrl.Result{}, nil
}
