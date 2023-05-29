package utils

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComposeRoleBinding returns a RoleBinding object based on the given parameters
func ComposeRoleBinding(rbName string, namespace string, subjects []rbacv1.Subject, ref rbacv1.RoleRef) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: namespace,
		},
		Subjects: subjects,
		RoleRef:  ref,
	}
}

// GetNSClusterRoleHNSViewName returns the name of the HNS view role associated with a namespace
func GetNSClusterRoleHNSViewName(nsName string) string {
	return nsName + "-hns-view"
}
