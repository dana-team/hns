package utils

import (
	"fmt"
	"reflect"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func isRoleBinding(sns client.Object) bool {
	if reflect.TypeOf(sns) == reflect.TypeOf(&rbacv1.RoleBinding{}) {
		return true
	}
	return false
}

func isClusterRoleBinding(sns client.Object) bool {
	if reflect.TypeOf(sns) == reflect.TypeOf(&rbacv1.ClusterRoleBinding{}) {
		return true
	}
	return false
}

func GetNamespaceRole(namespace client.Object) string {
	if !isNamespace(namespace) {
		return ""
	}
	return namespace.(*corev1.Namespace).Annotations[danav1.Role]
}

func IsValidRoleBinding(roleBinding client.Object) bool {
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
	if rbName == "default" || rbName == "builder" || rbName == "deployer" {
		return false
	}
	return true
}

func GetRoleBindingSubjects(roleBinding client.Object) []rbacv1.Subject {
	if !isRoleBinding(roleBinding) {
		return nil
	}
	if roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace == "" {
		roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace = roleBinding.(*rbacv1.RoleBinding).Namespace
	}
	return roleBinding.(*rbacv1.RoleBinding).Subjects
}

func GetRoleBindingRoleRef(roleBinding client.Object) rbacv1.RoleRef {
	if !isRoleBinding(roleBinding) {
		return rbacv1.RoleRef{}

	}
	return roleBinding.(*rbacv1.RoleBinding).RoleRef
}

func GetRoleBindingClusterRoleName(roleBinding client.Object) string {
	if !isRoleBinding(roleBinding) {
		return ""
	}
	name := roleBinding.(*rbacv1.RoleBinding).Namespace
	user := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Name
	//the name contain the crq name + the user name.
	return name + "-" + user
}

func GetRoleBindingSnsViewClusterRoleName(roleBinding client.Object) string {
	return fmt.Sprintf("%s-%s", GetRoleBindingClusterRoleName(roleBinding), "sns-view")
}

func IsRoleBindingFinalizerExists(roleBinding client.Object) bool {
	if !isRoleBinding(roleBinding) {
		return false
	}
	return controllerutil.ContainsFinalizer(roleBinding, danav1.RbFinalizer)
}

func GetNsHnsViewRoleName(namespaceName string) string {
	return namespaceName + "-hns-view"
}
