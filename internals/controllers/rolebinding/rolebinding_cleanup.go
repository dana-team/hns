package controllers

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cleanUp takes care of clean-up related operations that need to be done when
// a roleBinding relevant to HNS is deleted
func (r *RoleBindingReconciler) cleanUp(rbObject *utils.ObjectContext, sns *utils.ObjectContextList) error {
	ctx := rbObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("cleaning up roleBinding")

	rbName := rbObject.Object.GetName()
	rbNamespace := rbObject.Object.GetNamespace()

	if !isServiceAccount(rbObject.Object) {
		if err := deleteSubjectsFromHNSViewClusterRoleBinding(rbObject); err != nil {
			return fmt.Errorf("failed to delete subjects from roleBinding '%s' to HNS View ClusterRoleBinding: "+err.Error(), rbName)
		}
		logger.Info("successfully deleted subjects from roleBinding to HNS View ClusterRoleBinding", "roleBinding", rbName)
	}

	if err := deleteRoleBindingsInSnsList(rbObject, sns); err != nil {
		return fmt.Errorf("failed to delete RoleBinding in every child of namespace '%s': "+err.Error(), rbNamespace)
	}
	logger.Info("successfully deleted RoleBinding in every child of namespace", "roleBinding namespace", rbNamespace)

	if err := deleteRBFinalizer(rbObject); err != nil {
		return fmt.Errorf("failed to delete finalizer to roleBinding '%s': "+err.Error(), rbName)
	}
	logger.Info("successfully deleted finalizer to roleBinding", "roleBinding", rbName)

	return nil
}

// deleteSubjectsFromHNSViewClusterRoleBinding deletes any subjects existing in the reconciled
// roleBinding from the relevant HNS View CRB
func deleteSubjectsFromHNSViewClusterRoleBinding(rbObject *utils.ObjectContext) error {
	nsHNSViewClusterRoleBinding, err := getNamespaceHNSViewCRB(rbObject)
	if err != nil {
		return err
	}

	hnsViewSubjects := nsHNSViewClusterRoleBinding.Object.(*rbacv1.ClusterRoleBinding).Subjects

	var newSubjects []rbacv1.Subject
	roleBindingSubjects := rbObject.Object.(*rbacv1.RoleBinding).Subjects

	for _, subject := range hnsViewSubjects {
		if !isSubjectInSubjects(roleBindingSubjects, subject) {
			newSubjects = append(newSubjects, subject)
		}
	}

	return updateNamespaceHNSViewCRBSubjects(nsHNSViewClusterRoleBinding, newSubjects)
}

// deleteRoleBindingsInSNSList deletes the RoleBinding in every namespace linked to a subnamespace
// that is given as input in the snsList slice
func deleteRoleBindingsInSnsList(rbObject *utils.ObjectContext, snsList *utils.ObjectContextList) error {
	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		if err := deleteRoleBinding(rbObject, sns); err != nil {
			return err
		}
	}

	return nil
}

// deleteRoleBinding deletes a RoleBinding from a namespace
func deleteRoleBinding(rbObject *utils.ObjectContext, sns danav1.Subnamespace) error {
	rbName := rbObject.Object.GetName()

	rbSubjects := getRoleBindingSubjects(rbObject.Object)
	rbRoleRef := getRoleBindingRoleRef(rbObject.Object)

	rb := utils.ComposeRoleBinding(rbName, sns.Name, rbSubjects, rbRoleRef)
	roleBindingToDelete, err := utils.NewObjectContext(rbObject.Ctx, rbObject.Client, types.NamespacedName{Name: rbName, Namespace: sns.Name}, rb)
	if err != nil {
		return err
	}

	err = roleBindingToDelete.EnsureDeleteObject()

	return err
}

// deleteRBFinalizer deletes the HNS finalizer from a RoleBinding
func deleteRBFinalizer(roleBinding *utils.ObjectContext) error {
	return roleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("deleted roleBinding finalizer", danav1.RbFinalizer)
		controllerutil.RemoveFinalizer(object, danav1.RbFinalizer)
		return object, log
	})
}
