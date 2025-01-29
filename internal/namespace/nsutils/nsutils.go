package nsutils

import (
	"reflect"
	"strconv"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/objectcontext"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	trueString = "true"
)

// IsNamespace returns true if an object is of type namespace.
func IsNamespace(obj client.Object) bool {
	return reflect.TypeOf(obj) == reflect.TypeOf(&corev1.Namespace{})
}

// IsChildless checks if the given namespace has any subnamespaces,
// and returns true if it does not have any subnamespaces.
func IsChildless(namespace *objectcontext.ObjectContext) bool {
	snsList, err := objectcontext.NewList(namespace.Ctx, namespace.Client, &danav1.SubnamespaceList{}, client.InNamespace(namespace.Name()))
	if err != nil {
		return false
	}

	return len(snsList.Objects.(*danav1.SubnamespaceList).Items) == 0
}

// IsRoot returns true if the namespace is the root namespace.
func IsRoot(namespace client.Object) bool {
	return namespace.GetAnnotations()[danav1.Role] == danav1.Root
}

// DisplayName returns the displayName annotation of a namespace.
func DisplayName(namespace client.Object) string {
	if !IsNamespace(namespace) {
		return ""
	}
	return namespace.GetAnnotations()[danav1.DisplayName]
}

// Parent returns the parent label of a namespace.
func Parent(namespace client.Object) string {
	return namespace.GetLabels()[danav1.Parent]
}

// IsSecondaryRoot returns true if a namespace is a secondary root.
func IsSecondaryRoot(namespace client.Object) bool {
	return namespace.GetAnnotations()[danav1.IsSecondaryRoot] == danav1.True
}

// DisplayNameSlice returns a slice of strings that contains the hierarchy
// of a namespace in accordance to its display-name.
func DisplayNameSlice(ns *objectcontext.ObjectContext) []string {
	displayName := ns.Object.GetAnnotations()[danav1.DisplayName]
	nsArr := strings.Split(displayName, "/")

	return nsArr
}

// LabelsBasedOnParent returns labels for a namespace based on the ones of the namespace of the parent.
func LabelsBasedOnParent(parentNS *objectcontext.ObjectContext, nsName string) map[string]string {
	parentDisplayNameSliced := DisplayNameSlice(parentNS)

	labels := make(map[string]string)
	defaultLabels(parentNS, labels)

	labels[danav1.Parent] = parentNS.Object.(*corev1.Namespace).Name
	labels[danav1.Hns] = trueString

	for _, ns := range parentDisplayNameSliced {
		labels[ns] = trueString
	}
	labels[nsName] = trueString

	return labels
}

// AnnotationsBasedOnParent returns labels for a namespace based on the ones of the namespace of the parent.
func AnnotationsBasedOnParent(parentNS *objectcontext.ObjectContext, nsName string) map[string]string {
	childNamespaceDepth := strconv.Itoa(Depth(parentNS.Object) + 1)
	parentDisplayName := DisplayName(parentNS.Object)

	annotations := crqSelectors(parentNS)
	defaultAnnotations(parentNS, annotations)

	annotations[danav1.Depth] = childNamespaceDepth
	annotations[danav1.DisplayName] = parentDisplayName + "/" + nsName
	annotations[danav1.OpenShiftDisplayName] = parentDisplayName + "/" + nsName
	annotations[danav1.SnsPointer] = nsName
	annotations[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName

	return annotations
}

// crqSelectors returns a slice of crq selectors.
func crqSelectors(ns *objectcontext.ObjectContext) map[string]string {
	selectors := map[string]string{}

	for key, value := range ns.Object.GetAnnotations() {
		if strings.Contains(key, danav1.CrqSelector) {
			selectors[key] = value
		}
	}
	return selectors
}

// defaultAnnotations updates the map of the ns annotations with the DefaultAnnotations.
func defaultAnnotations(ns *objectcontext.ObjectContext, annotations map[string]string) {
	for key, value := range ns.Object.GetAnnotations() {
		if slices.Contains(danav1.DefaultAnnotations, key) {
			annotations[key] = value
		}
	}
}

// defaultLabels updates the map of the ns labels with the DefaultLabels.
func defaultLabels(ns *objectcontext.ObjectContext, labels map[string]string) {
	for key, value := range ns.Object.GetLabels() {
		if slices.Contains(danav1.DefaultLabels, key) {
			labels[key] = value
		}
	}
}

// Depth returns the depth of a namespace from its annotation.
func Depth(namespace client.Object) int {
	if ownerNamespaceDepth, err := strconv.Atoi(namespace.(*corev1.Namespace).Annotations[danav1.Depth]); err != nil {
		return 0
	} else {
		return ownerNamespaceDepth
	}
}

// SNSFromNamespace returns the subnamespace corresponding to a namespace.
func SNSFromNamespace(namespace *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	if !IsNamespace(namespace.Object) {
		return nil, nil
	}

	if IsRoot(namespace.Object) {
		return nil, nil
	}

	sns, err := objectcontext.New(namespace.Ctx, namespace.Client, types.NamespacedName{Name: namespace.Name(), Namespace: Parent(namespace.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return nil, err
	}

	return sns, nil
}

// ComposeNamespace returns a namespace object based on the given parameters.
func ComposeNamespace(name string, labels map[string]string, annotations map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}
