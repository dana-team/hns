package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ComposeNamespace(name string, labels map[string]string, annotations map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func ComposeResourceQuota(name string, namespace string, hard corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		TypeMeta: metav1.TypeMeta{
			Kind: "ResourceQuota",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}
}

func ComposeLimitRange(name string, namespace string, limits corev1.LimitRangeItem) *corev1.LimitRange {
	return &corev1.LimitRange{
		TypeMeta: metav1.TypeMeta{
			Kind: "LimitRange",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				limits,
			},
		},
	}
}

func ComposeCrq(name string, quota corev1.ResourceQuotaSpec, annSelector map[string]string) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"crq.subnamespace": name},
		},
		Spec: quotav1.ClusterResourceQuotaSpec{
			Selector: quotav1.ClusterResourceQuotaSelector{
				AnnotationSelector: annSelector,
			},
			Quota: quota,
		},
	}
}

func ComposeRq(name string, quota corev1.ResourceQuotaSpec) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"crq.subnamespace": name},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: quota.Hard,
		},
	}
}

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

func ComposeSns(name string, namespace string, quota corev1.ResourceList, labels map[string]string) *danav1.Subnamespace {
	return &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: danav1.SubnamespaceSpec{
			ResourceQuotaSpec: corev1.ResourceQuotaSpec{Hard: quota}},
	}
}

func ComposeClusterRoleBinding(roleBinding client.Object, name string) *rbacv1.ClusterRoleBinding {
	if !isRoleBinding(roleBinding) {
		return nil
	}
	//the name contain the crq name + the user name.
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: roleBinding.(*rbacv1.RoleBinding).Subjects,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
	}
}

func ComposeSnsViewClusterRole(roleBinding client.Object) *rbacv1.ClusterRole {
	if !isRoleBinding(roleBinding) {
		return nil
	}
	name := roleBinding.(*rbacv1.RoleBinding).Namespace
	//the name contain the crq name + the user name.
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetRoleBindingSnsViewClusterRoleName(roleBinding),
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get"},
			APIGroups: []string{"dana.hns.io"},
			Resources: []string{"subnamespaces"},
			//the crq is always the same as the ns name
			ResourceNames: []string{name}}},
	}
}

func ComposeNsHnsViewClusterRole(namespaceName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetNsHnsViewRoleName(namespaceName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"dana.hns.io"},
				Resources: []string{"subnamespaces"},

				//the crq/rq is always the same as the ns name
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

func ComposeNsHnsViewClusterRoleBinding(namespaceName string) *rbacv1.ClusterRoleBinding {
	name := GetNsHnsViewRoleName(namespaceName)
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

func ComposeClusterRole(roleBinding client.Object) *rbacv1.ClusterRole {
	if !isRoleBinding(roleBinding) {
		return nil
	}
	name := roleBinding.(*rbacv1.RoleBinding).Namespace
	//the name contain the crq name + the user name.
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetRoleBindingClusterRoleName(roleBinding),
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get"},
			APIGroups: []string{"quota.openshift.io"},
			Resources: []string{"clusterresourcequotas"},
			//the crq is always the same as the ns name
			ResourceNames: []string{name}},

			{Verbs: []string{"list"},
				APIGroups: []string{"quota.openshift.io"},
				Resources: []string{"clusterresourcequotas"}}},
	}
}
