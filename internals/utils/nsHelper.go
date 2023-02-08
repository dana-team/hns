package utils

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

func GetNsListUp(ns *ObjectContext, rootns string, rclient client.Client, logger logr.Logger) ([]*ObjectContext, error) {

	var nsList []*ObjectContext

	displayName := ns.Object.GetAnnotations()["openshift.io/display-name"]
	nsArr := strings.Split(displayName, "/")
	index, err := IndexOf(rootns, nsArr)
	if err != nil {
		return nil, err
	}
	snsArr := nsArr[index:]

	for i := len(snsArr) - 1; i >= 1; i-- {
		ns, err := NewObjectContext(context.Background(), logger.WithValues("get ns list", ""), rclient, client.ObjectKey{Name: snsArr[i]}, &corev1.Namespace{})
		if err != nil {
			return nil, err
		}
		nsList = append(nsList, ns)
	}

	return nsList, nil
}

func GetRootns(ns *ObjectContext) (*ObjectContext, error) {
	nsDisplayName := ns.Object.GetAnnotations()["openshift.io/display-name"]
	nsArr := strings.Split(nsDisplayName, "/")
	rootNamespace, err := NewObjectContext(ns.Ctx, ns.Log, ns.Client, types.NamespacedName{Name: nsArr[0]}, &corev1.Namespace{})
	if err != nil {
		return ns, err
	}
	return rootNamespace, nil
}

func GetNamespacerqDepth(namespace client.Object) int {
	if !isNamespace(namespace) {
		return 0
	}

	if ownerNamespaceDepth, err := strconv.Atoi(namespace.(*corev1.Namespace).Annotations[danav1.RqDepth]); err != nil {
		return 0
	} else {
		return ownerNamespaceDepth
	}
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
