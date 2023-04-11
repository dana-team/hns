package utils

import (
	"reflect"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func isNamespace(namespace client.Object) bool {
	if reflect.TypeOf(namespace) == reflect.TypeOf(&corev1.Namespace{}) {
		return true
	}
	return false
}

func IsRootNamespace(namespace client.Object) bool {
	if !isNamespace(namespace) {
		return false
	}
	return namespace.(*corev1.Namespace).Annotations[danav1.Role] == danav1.Root
}

func GetNamespaceDisplayName(namespace client.Object) string {
	if !isNamespace(namespace) {
		return ""
	}
	return namespace.(*corev1.Namespace).Annotations["openshift.io/display-name"]
}

func IsNamespaceFinalizerExists(namespace client.Object) bool {
	return controllerutil.ContainsFinalizer(namespace, danav1.NsFinalizer)
}

func NamespaceFinalizerExists(namespace client.Object) bool {
	return controllerutil.ContainsFinalizer(namespace, danav1.NsFinalizer)
}

func LocateNS(nsList corev1.NamespaceList, nsName string) *corev1.Namespace {
	for _, ns := range nsList.Items {
		if ns.Name == nsName {
			return &ns
		}
	}
	return nil
}

func GetNsListUpEfficient(ns corev1.Namespace, rootns string, nsList corev1.NamespaceList) ([]corev1.Namespace, error) {

	var nsListUp []corev1.Namespace

	displayName := ns.GetAnnotations()["openshift.io/display-name"]
	nsArr := strings.Split(displayName, "/")
	index, err := IndexOf(rootns, nsArr)
	if err != nil {
		return nil, err
	}
	snsArr := nsArr[index:]

	for i := len(snsArr) - 1; i >= 1; i-- {
		ns := LocateNS(nsList, snsArr[i])
		nsListUp = append(nsListUp, *ns)
	}

	return nsListUp, nil
}

func GetNamespaceParent(namespace client.Object) string {
	if !isNamespace(namespace) {
		return ""
	}
	return namespace.(*corev1.Namespace).Labels[danav1.Parent]
}

func IsSecondaryRootNamespace(namespace client.Object) bool {
	if !isNamespace(namespace) {
		return false
	}
	return namespace.(*corev1.Namespace).Annotations[danav1.IsSecondaryRoot] == danav1.True
}
