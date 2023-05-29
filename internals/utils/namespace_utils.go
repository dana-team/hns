package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

// isNamespace returns true if an object is of type namespace
func isNamespace(obj client.Object) bool {
	return reflect.TypeOf(obj) == reflect.TypeOf(&corev1.Namespace{})
}

// IsChildlessNamespace checks if the given namespace has any subnamespaces,
// and returns true if it does not have any subnamespaces
func IsChildlessNamespace(namespace *ObjectContext) bool {
	snsList, err := NewObjectContextList(namespace.Ctx, namespace.Client, &danav1.SubnamespaceList{}, client.InNamespace(namespace.Object.GetName()))
	if err != nil {
		return false
	}

	return len(snsList.Objects.(*danav1.SubnamespaceList).Items) == 0
}

// IsRootNamespace returns true if the namespace is the root namespace
func IsRootNamespace(namespace client.Object) bool {
	return namespace.GetAnnotations()[danav1.Role] == danav1.Root
}

// GetNamespaceDisplayName returns the displayName annotation of a namespace
func GetNamespaceDisplayName(namespace client.Object) string {
	if !isNamespace(namespace) {
		return ""
	}
	return namespace.GetAnnotations()[danav1.DisplayName]
}

// GetNamespaceParent returns the parent label of a namespace
func GetNamespaceParent(namespace client.Object) string {
	return namespace.GetLabels()[danav1.Parent]
}

// IsSecondaryRootNamespace returns true if a namespace is a secondary root
func IsSecondaryRootNamespace(namespace client.Object) bool {
	return namespace.GetAnnotations()[danav1.IsSecondaryRoot] == danav1.True
}

// GetNSDisplayNameSlice returns a slice of strings that contains the hierarchy
// of a namespace in accordance to its display-name
func GetNSDisplayNameSlice(ns *ObjectContext) []string {
	displayName := ns.Object.GetAnnotations()[danav1.DisplayName]
	nsArr := strings.Split(displayName, "/")

	return nsArr
}

// GetNSLabelsAnnotationsBasedOnParent returns labels and annotations for a namespace,
// the labels and annotations are based on the ones of the namespace of the parent
func GetNSLabelsAnnotationsBasedOnParent(parentNS *ObjectContext, nsName string) (map[string]string, map[string]string) {
	childNamespaceDepth := strconv.Itoa(GetNamespaceDepth(parentNS.Object) + 1)
	parentDisplayName := GetNamespaceDisplayName(parentNS.Object)

	labels := make(map[string]string)
	labels[danav1.Parent] = parentNS.Object.(*corev1.Namespace).Name
	labels[danav1.Hns] = "true"

	annotations := GetNSCrqSelectors(parentNS)

	annotations[danav1.Depth] = childNamespaceDepth
	annotations[danav1.DisplayName] = parentDisplayName + "/" + nsName
	annotations[danav1.OpenShiftDisplayName] = parentDisplayName + "/" + nsName
	annotations[danav1.SnsPointer] = nsName
	annotations[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName

	return labels, annotations
}

// GetNSCrqSelectors returns a slice of crq selectors
func GetNSCrqSelectors(ns *ObjectContext) map[string]string {
	selectors := map[string]string{}

	for key, value := range ns.Object.GetAnnotations() {
		if strings.Contains(key, danav1.CrqSelector) {
			selectors[key] = value
		}
	}
	return selectors
}

// GetNamespaceDepth returns the depth of a namespace from its annotation
func GetNamespaceDepth(namespace client.Object) int {
	if ownerNamespaceDepth, err := strconv.Atoi(namespace.(*corev1.Namespace).Annotations[danav1.Depth]); err != nil {
		return 0
	} else {
		return ownerNamespaceDepth
	}
}
