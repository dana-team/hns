package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetNamespaceResourcePooled(namespace *ObjectContext) string {
	if !isNamespace(namespace.Object) {
		return "false"
	}
	if IsRootNamespace(namespace.Object) {
		return "false"
	}
	namespaceSns, err := GetNamespaceSns(namespace)
	if err != nil {
		return "false"
	}
	currentState := namespaceSns.Object.(*danav1.Subnamespace).Labels[danav1.ResourcePool]
	if currentState == "" {
		return "false"
	}
	return currentState
}

func GetSnsResourcePooled(sns client.Object) string {
	if !isSns(sns) {
		return ""
	}
	return sns.(*danav1.Subnamespace).Labels[danav1.ResourcePool]
}

// RqcEqual gets two ResourceQuotaSpecs and returns whether their specs are equal
func RqcEqual(a, b corev1.ResourceQuotaSpec) bool {
	if *b.Hard.Cpu() != *a.Hard.Cpu() ||
		*b.Hard.Memory() != *a.Hard.Memory() ||
		*b.Hard.Pods() != *a.Hard.Pods() ||
		*b.Hard.Storage() != *a.Hard.Storage() {
		return false
	}
	return true
}

// NamespacesEqual gets two []danav1.Namespaces and returns whether they are equal
func NamespacesEqual(a, b []danav1.Namespaces) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if !RqcEqual(v.ResourceQuotaSpec, b[i].ResourceQuotaSpec) {
			return false
		}
	}
	return true
}

// ResourceListEqual gets two ResourceLists and returns whether their specs are equal
func ResourceListEqual(a, b corev1.ResourceList) bool {
	if !b.Cpu().Equal(*a.Cpu()) || !b.Memory().Equal(*a.Memory()) ||
		!b.Pods().Equal(*a.Pods()) || !b.Storage().Equal(*a.Storage()) {
		return false
	}
	return true
}

// IsChildUpperRp gets a subnamespace father and child objects and returns whether the child should now become
// the upper resource pool
func IsChildUpperRp(father, child client.Object) bool {
	if GetSnsResourcePooled(father) == "false" && GetSnsResourcePooled(child) == "true" &&
		child.GetAnnotations()[danav1.IsUpperRp] == danav1.False {
		return true
	}
	return false
}

// IsUpdateNeeded gets a subnamespace object, a []danav1.Namespaces and two resource lists and returns whether
// the subnamespace object status has to be updated
func IsUpdateNeeded(subspace client.Object, childrenRequests []danav1.Namespaces, allocated, free corev1.ResourceList) bool {
	if !NamespacesEqual(subspace.(*danav1.Subnamespace).Status.Namespaces, childrenRequests) ||
		!ResourceListEqual(subspace.(*danav1.Subnamespace).Status.Total.Allocated, allocated) ||
		!ResourceListEqual(subspace.(*danav1.Subnamespace).Status.Total.Free, free) {
		return true
	}
	return false
}
