package controllers

import (
	"github.com/dana-team/hns/internals/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// getNamespaceHNSView returns the cluster role and cluster role binding HNS objects
// associated with the namespace
func getNamespaceHNSView(nsObject *utils.ObjectContext) (*utils.ObjectContext, *utils.ObjectContext, error) {
	nsClusterRoleName := utils.GetNSClusterRoleHNSViewName(nsObject.Object.GetName())
	nsClusterRole := composeNsHnsViewClusterRole(nsObject.Object.GetName())
	nsClusterRoleBinding := composeNsHnsViewClusterRoleBinding(nsObject.Object.GetName())

	nsHnsViewClusterRoleObj, err := utils.NewObjectContext(nsObject.Ctx, nsObject.Client, types.NamespacedName{Name: nsClusterRoleName}, nsClusterRole)
	if err != nil {
		return nil, nil, err
	}

	nsHnsViewClusterRoleBindingObj, err := utils.NewObjectContext(nsObject.Ctx, nsObject.Client, types.NamespacedName{Name: nsClusterRoleName}, nsClusterRoleBinding)
	if err != nil {
		return nil, nil, err
	}

	return nsHnsViewClusterRoleObj, nsHnsViewClusterRoleBindingObj, nil
}

// createNamespaceHNSView creates the cluster role and cluster role binding HNS objects
// associated with the namespace
func createNamespaceHNSView(nsObject *utils.ObjectContext) error {
	nsHnsViewClusterRoleObj, nsHnsViewClusterRoleBindingObj, err := getNamespaceHNSView(nsObject)
	if err != nil {
		return err
	}

	if err := nsHnsViewClusterRoleObj.EnsureCreateObject(); err != nil {
		return err
	}

	return nsHnsViewClusterRoleBindingObj.EnsureCreateObject()
}

// composeNsHnsViewClusterRole returns a ClusterRole object based on the given parameters
func composeNsHnsViewClusterRole(namespaceName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.GetNSClusterRoleHNSViewName(namespaceName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"dana.hns.io"},
				Resources: []string{"subnamespaces"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"get"},
				APIGroups: []string{"quota.openshift.io"},
				Resources: []string{"clusterresourcequotas"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"list"},
				APIGroups: []string{"quota.openshift.io"},
				Resources: []string{"clusterresourcequotas"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"get"},
				APIGroups: []string{"core"},
				Resources: []string{"resourcequotas"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"list"},
				APIGroups: []string{"core"},
				Resources: []string{"resourcequotas"},
			},
		},
	}
}

// composeNsHnsViewClusterRoleBinding returns a ClusterRoleBinding object based on the given parameters
func composeNsHnsViewClusterRoleBinding(namespaceName string) *rbacv1.ClusterRoleBinding {
	name := utils.GetNSClusterRoleHNSViewName(namespaceName)
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
	}
}

// createRoleBinding creates a RoleBinding object in a namespace from an existing roleBinding
func createRoleBinding(nsObject *utils.ObjectContext, roleBinding rbacv1.RoleBinding) error {
	rb := utils.ComposeRoleBinding(roleBinding.Name, nsObject.Object.GetName(), roleBinding.Subjects, roleBinding.RoleRef)

	rbObject, err := utils.NewObjectContext(nsObject.Ctx, nsObject.Client, types.NamespacedName{}, rb)
	if err != nil {
		return err
	}

	err = rbObject.CreateObject()

	return err
}
