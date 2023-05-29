package webhooks

import (
	"fmt"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handle implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request)
func (a *RoleBindingAnnotator) handle(rbObject *utils.ObjectContext) admission.Response {
	ctx := rbObject.Ctx
	logger := log.FromContext(ctx)

	rbNamespace := rbObject.Object.GetNamespace()
	rbName := rbObject.Object.GetName()

	namespace, err := utils.NewObjectContext(ctx, a.Client, types.NamespacedName{Name: rbNamespace}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "roleBinding Namespace", rbNamespace)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := a.validateNamespaceDeletion(namespace); response.Allowed {
		return response
	}

	if response := a.validateParentRoleBinding(namespace, rbName); response.Allowed {
		return response
	}

	message := fmt.Sprintf("it's forbidden to delete a RoleBinding not at the top of the hierarchy."+
		"Delete the RoleBinding '%s' in the highest hierarchy it exists", rbName)
	return admission.Denied(message)
}

// validateNamespaceDeletion validates if a namespace is being deleted
func (a *RoleBindingAnnotator) validateNamespaceDeletion(ns *utils.ObjectContext) admission.Response {
	if utils.DeletionTimeStampExists(ns.Object) {
		return admission.Allowed("it is allowed to delete the RoleBinding because the Namespace it's in is being deleted")
	}

	return admission.Denied("")
}

// validateParentRoleBinding validates the state of the parent RoleBinding
func (a *RoleBindingAnnotator) validateParentRoleBinding(ns *utils.ObjectContext, name string) admission.Response {
	logger := log.FromContext(ns.Ctx)
	rbParentNS := utils.GetNamespaceParent(ns.Object)

	parentRoleBinding, err := utils.NewObjectContext(ns.Ctx, a.Client, types.NamespacedName{Namespace: rbParentNS, Name: name}, &rbacv1.RoleBinding{})
	if err != nil {
		logger.Error(err, "failed to create object", "parent roleBinding", name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if utils.DeletionTimeStampExists(parentRoleBinding.Object) {
		return admission.Allowed("it is allowed to delete the RoleBinding because its parent RoleBinding is being deleted")
	}

	if !parentRoleBinding.IsPresent() {
		return admission.Allowed("it is allowed to delete the RoleBinding because its parent RoleBinding doesn't exist")
	}

	return admission.Denied("")
}
