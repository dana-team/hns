package rolebinding

import (
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/rolebinding/rbutils"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cleanUp takes care of clean-up related operations that need to be done when
// a roleBinding relevant to HNS is deleted.
func (r *RoleBindingReconciler) cleanUp(rbObject *objectcontext.ObjectContext, snsList *objectcontext.ObjectContextList) error {
	ctx := rbObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("cleaning up roleBinding")

	rbName := rbObject.Name()
	rbNamespace := rbObject.Object.GetNamespace()

	if !rbutils.IsServiceAccount(rbObject.Object) {
		if err := deleteSubjectsFromHNSViewClusterRoleBinding(rbObject); err != nil {
			return fmt.Errorf("failed to delete subjects from roleBinding %q to HNS View ClusterRoleBinding: "+err.Error(), rbName)
		}
		logger.Info("successfully deleted subjects from roleBinding to HNS View ClusterRoleBinding", "roleBinding", rbName)
	}

	if err := deleteRoleBindingsInSnsList(rbObject, snsList); err != nil {
		return fmt.Errorf("failed to delete RoleBinding in every child of namespace %q: "+err.Error(), rbNamespace)
	}
	logger.Info("successfully deleted RoleBinding in every child of namespace", "roleBinding namespace", rbNamespace)

	if err := deleteRBFinalizer(rbObject); err != nil {
		return fmt.Errorf("failed to delete finalizer to roleBinding %q: "+err.Error(), rbName)
	}
	logger.Info("successfully deleted finalizer to roleBinding", "roleBinding", rbName)

	return nil
}

// deleteSubjectsFromHNSViewClusterRoleBinding deletes any subjects existing in the reconciled
// roleBinding from the relevant HNS View CRB.
func deleteSubjectsFromHNSViewClusterRoleBinding(rbObject *objectcontext.ObjectContext) error {
	nsHNSViewClusterRoleBinding, err := rbutils.NamespaceHNSViewCRB(rbObject)
	if err != nil {
		return err
	}

	hnsViewSubjects := nsHNSViewClusterRoleBinding.Object.(*rbacv1.ClusterRoleBinding).Subjects

	var newSubjects []rbacv1.Subject
	roleBindingSubjects := rbObject.Object.(*rbacv1.RoleBinding).Subjects

	for _, subject := range hnsViewSubjects {
		if !rbutils.IsSubjectInSubjects(roleBindingSubjects, subject) {
			newSubjects = append(newSubjects, subject)
		}
	}

	return rbutils.UpdateNamespaceHNSViewCRBSubjects(nsHNSViewClusterRoleBinding, newSubjects)
}

// deleteRoleBindingsInSNSList deletes the RoleBinding in every namespace linked to a subnamespace
// that is given as input in the snsList slice.
func deleteRoleBindingsInSnsList(rbObject *objectcontext.ObjectContext, snsList *objectcontext.ObjectContextList) error {
	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		if err := deleteRoleBinding(rbObject, sns); err != nil {
			return err
		}
	}

	return nil
}

// deleteRoleBinding deletes a RoleBinding from a namespace.
func deleteRoleBinding(rbObject *objectcontext.ObjectContext, sns danav1.Subnamespace) error {
	rbName := rbObject.Name()

	rbSubjects := rbutils.Subjects(rbObject.Object)
	rbRoleRef := rbutils.RoleRef(rbObject.Object)

	rb := rbutils.Compose(rbName, sns.Name, rbSubjects, rbRoleRef)
	roleBindingToDelete, err := objectcontext.New(rbObject.Ctx, rbObject.Client, types.NamespacedName{Name: rbName, Namespace: sns.Name}, rb)
	if err != nil {
		return err
	}

	err = roleBindingToDelete.EnsureDelete()

	return err
}

// deleteRBFinalizer deletes the HNS finalizer from a RoleBinding.
func deleteRBFinalizer(roleBinding *objectcontext.ObjectContext) error {
	return roleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("deleted roleBinding finalizer", danav1.RbFinalizer)
		controllerutil.RemoveFinalizer(object, danav1.RbFinalizer)
		return object, log
	})
}
