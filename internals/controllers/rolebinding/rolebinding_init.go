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

// init takes care of initializing related operations that need to be done when
// a namespace is reconciled for the first time
func (r *RoleBindingReconciler) init(rbObject *utils.ObjectContext, snsList *utils.ObjectContextList) error {
	ctx := rbObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("initializing roleBinding")

	rbName := rbObject.Object.GetName()
	rbNamespace := rbObject.Object.GetNamespace()

	if !DoesRBFinalizerExist(rbObject.Object) {
		if err := addRBFinalizer(rbObject); err != nil {
			return fmt.Errorf("failed to add finalizer to roleBinding '%s': "+err.Error(), rbName)
		}
		logger.Info("successfully added finalizer to roleBinding", "roleBinding", rbName)
	}

	if err := createRoleBindingsInSNSList(rbObject, snsList); err != nil {
		return fmt.Errorf("failed to create RoleBinding in every child of namespace '%s': "+err.Error(), rbNamespace)
	}
	logger.Info("successfully created RoleBinding in every child of namespace", "roleBinding namespace", rbNamespace)

	if !isRoleBindingHNSRelated(rbObject.Object) {
		if err := addSubjectsToHNSViewClusterRoleBinding(rbObject); err != nil {
			return fmt.Errorf("failed to add subjects from roleBinding '%s' to HNS View ClusterRoleBinding: "+err.Error(), rbName)
		}
		logger.Info("successfully added subjects from roleBinding to HNS View ClusterRoleBinding", "roleBinding", rbName)
	}

	return nil
}

// DoesRBFinalizerExist returns true if a roleBinding has the HNS finalizer
func DoesRBFinalizerExist(roleBinding client.Object) bool {
	if !isRoleBinding(roleBinding) {
		return false
	}
	return controllerutil.ContainsFinalizer(roleBinding, danav1.RbFinalizer)
}

// addNSFinalizer adds the HNS finalizer to a RoleBinding
func addRBFinalizer(roleBinding *utils.ObjectContext) error {
	return roleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("added roleBinding finalizer", danav1.RbFinalizer)
		controllerutil.AddFinalizer(object, danav1.RbFinalizer)
		return object, log
	})
}

// createRoleBindingsInSNSList creates the RoleBinding in every namespace linked to a subnamespace
// that is given as input in the snsList slice
func createRoleBindingsInSNSList(rbObject *utils.ObjectContext, snsList *utils.ObjectContextList) error {
	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		if err := createRoleBinding(rbObject, sns); err != nil {
			return err
		}
	}
	return nil
}

// createRoleBinding creates a RoleBinding in a namespace
func createRoleBinding(rbObject *utils.ObjectContext, sns danav1.Subnamespace) error {
	rbName := rbObject.Object.GetName()

	rbSubjects := getRoleBindingSubjects(rbObject.Object)
	rbRoleRef := getRoleBindingRoleRef(rbObject.Object)

	rb := utils.ComposeRoleBinding(rbName, sns.Name, rbSubjects, rbRoleRef)
	roleBindingToCreate, err := utils.NewObjectContext(rbObject.Ctx, rbObject.Client, types.NamespacedName{Name: rbName, Namespace: sns.Name}, rb)
	if err != nil {
		return err
	}

	err = roleBindingToCreate.EnsureCreateObject()

	return err
}

// addSubjectsToHNSViewClusterRoleBinding adds any subjects existing in the reconciled
// roleBinding but missing from the relevant HNS View CRB to that ClusterRoleBinding
func addSubjectsToHNSViewClusterRoleBinding(rbObject *utils.ObjectContext) error {
	nsHNSViewClusterRoleBinding, err := getNamespaceHNSViewCRB(rbObject)
	if err != nil {
		return err
	}

	hnsViewSubjects := nsHNSViewClusterRoleBinding.Object.(*rbacv1.ClusterRoleBinding).Subjects

	for _, subject := range rbObject.Object.(*rbacv1.RoleBinding).Subjects {
		if !isSubjectInSubjects(hnsViewSubjects, subject) {
			hnsViewSubjects = append(hnsViewSubjects, subject)
		}
	}
	return updateNamespaceHNSViewCRBSubjects(nsHNSViewClusterRoleBinding, hnsViewSubjects)
}
