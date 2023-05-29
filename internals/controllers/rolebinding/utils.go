package controllers

import (
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	defaultSA  = "default"
	builderSA  = "builder"
	deployerSA = "deployer"
)

// isRoleBindingHNSRelated checks if a RoleBinding object is valid.
func isRoleBindingHNSRelated(roleBinding client.Object) bool {
	if !isRoleBinding(roleBinding) {
		return false
	}

	if len(roleBinding.(*rbacv1.RoleBinding).Subjects) == 0 {
		return false
	}

	rbKind := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Kind
	if rbKind == "Group" {
		rbName := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Name
		if strings.HasPrefix(rbName, "system") {
			return false
		}
	}

	if rbKind == "ServiceAccount" {
		if roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace == "" {
			return false
		}
	}

	rbName := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Name
	serviceAccountNames := []string{defaultSA, builderSA, deployerSA}
	if utils.ContainsString(serviceAccountNames, rbName) {
		return false
	}

	return true
}

// isRoleBinding returns true if an object is of type RoleBinding
func isRoleBinding(object client.Object) bool {
	return reflect.TypeOf(object) == reflect.TypeOf(&rbacv1.RoleBinding{})
}

// getRoleBindingSubjects returns a slice of subjects from a roleBinding
func getRoleBindingSubjects(roleBinding client.Object) []rbacv1.Subject {
	if !isRoleBinding(roleBinding) {
		return nil
	}

	if roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace == "" {
		roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace = roleBinding.(*rbacv1.RoleBinding).Namespace
	}

	return roleBinding.(*rbacv1.RoleBinding).Subjects
}

// getRoleBindingRoleRef returns the roleRef of a roleBinding
func getRoleBindingRoleRef(roleBinding client.Object) rbacv1.RoleRef {
	if !isRoleBinding(roleBinding) {
		return rbacv1.RoleRef{}
	}

	return roleBinding.(*rbacv1.RoleBinding).RoleRef
}

// isServiceAccount returns true if a subject in a RoleBinding is a ServiceAccount and false otherwise
func isServiceAccount(roleBinding client.Object) bool {
	if !isRoleBindingHNSRelated(roleBinding) {
		return false
	}

	rbKind := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Kind
	if rbKind != "ServiceAccount" {
		return false
	}

	return true
}

// updateNamespaceHNSViewCRBSubjects updates the subjects of a ClusterRoleBinding
func updateNamespaceHNSViewCRBSubjects(hnsViewClusterRoleBinding *utils.ObjectContext, subjects []rbacv1.Subject) error {
	return hnsViewClusterRoleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated ClusterRoleBinding subjects", hnsViewClusterRoleBinding.Object.GetName())
		hnsViewClusterRoleBinding.Object.(*rbacv1.ClusterRoleBinding).Subjects = subjects
		return object, log
	})
}

// getNamespaceHNSView returns the HNS View ClusterRoleBinding associated with a namespace
func getNamespaceHNSViewCRB(rbObject *utils.ObjectContext) (*utils.ObjectContext, error) {
	namespace := rbObject.Object.GetNamespace()
	nsClusterRoleBindingName := utils.GetNSClusterRoleHNSViewName(namespace)

	nsClusterRoleBinding, err := utils.NewObjectContext(rbObject.Ctx, rbObject.Client, types.NamespacedName{Name: nsClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{})
	if err != nil {
		return nil, err
	}

	return nsClusterRoleBinding, nil
}

// isSubjectInSubjects returns whether a subject is in a slice of subjects
func isSubjectInSubjects(subjects []rbacv1.Subject, subjectToFind rbacv1.Subject) bool {
	for _, subject := range subjects {
		if subjectsEqual(subject, subjectToFind) {
			return true
		}
	}

	return false
}

// subjectsEqual returns true if two subjects are equal and false otherwise
func subjectsEqual(subject1 rbacv1.Subject, subject2 rbacv1.Subject) bool {
	return subject1.Name == subject2.Name && subject1.Kind == subject2.Kind && subject1.APIGroup == subject1.APIGroup
}
