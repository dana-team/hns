package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsRootResourcePool(sns *ObjectContext) bool {
	if !isSns(sns.Object) {
		return false
	}
	parentNamespace, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetNamespace()}, &corev1.Namespace{})
	if err != nil {
		return false
	}

	if sns.Object.(*danav1.Subnamespace).Labels[danav1.ResourcePool] == "true" {
		if len(GetSnsQuotaSpec(sns.Object).Hard) > 0 {
			if GetNamespaceResourcePooled(parentNamespace) == "false" {
				return true
			}
		}
	}
	return false
}

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
