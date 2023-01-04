package utils

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isSns(sns client.Object) bool {
	if reflect.TypeOf(sns) == reflect.TypeOf(&danav1.Subnamespace{}) {
		return true
	}
	return false
}

func GetNamespaceDepth(namespace client.Object) int {
	if !isNamespace(namespace) {
		return 0
	}

	if ownerNamespaceDepth, err := strconv.Atoi(namespace.(*corev1.Namespace).Annotations[danav1.Depth]); err != nil {
		return 0
	} else {
		return ownerNamespaceDepth
	}
}

func GetSnsOwner(sns client.Object) string {
	if !isSns(sns) {
		return ""
	}
	return sns.(*danav1.Subnamespace).GetNamespace()
}

func GetNamespaceSns(namespace *ObjectContext) (*ObjectContext, error) {
	if !isNamespace(namespace.Object) {
		return nil, nil
	}
	if IsRootNamespace(namespace.Object) {
		return nil, nil
	}
	namespaceSns, err := NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Name: namespace.Object.GetName(), Namespace: GetNamespaceParent(namespace.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return nil, err
	}
	return namespaceSns, nil
}

func GetSnsPhase(sns client.Object) danav1.Phase {
	if !isSns(sns) {
		return ""
	}
	return sns.(*danav1.Subnamespace).Status.Phase
}

func GetNamespaceSnsPointer(namespace client.Object) string {
	if !isNamespace(namespace) {
		return ""
	}
	return namespace.(*corev1.Namespace).Annotations[danav1.SnsPointer]
}

func GetSnsNamespaceRef(sns client.Object) string {
	if !isSns(sns) {
		return ""
	}
	return sns.(*danav1.Subnamespace).Spec.NamespaceRef.Name
}

func GetSnsListUp(ns *ObjectContext, rootns string, rclient client.Client, logger logr.Logger) ([]*ObjectContext, error) {

	var snsList []*ObjectContext

	displayName := ns.Object.GetAnnotations()["openshift.io/display-name"]
	nsArr := strings.Split(displayName, "/")
	index, err := IndexOf(rootns, nsArr)
	if err != nil {
		return nil, err
	}
	snsArr := nsArr[index:]

	for i := len(snsArr) - 1; i >= 1; i-- {
		sns, err := NewObjectContext(context.Background(), logger.WithValues("get sns list", ""), rclient, client.ObjectKey{Name: snsArr[i], Namespace: snsArr[i-1]}, &danav1.Subnamespace{})
		if err != nil {
			return nil, err
		}
		snsList = append(snsList, sns)
	}

	return snsList, nil
}

// GetNSDisplayNameArray returns an array of strings that contains the hierarchy
// of a namespace in accordance to its display-name
func GetNSDisplayNameArray(ns *ObjectContext) []string {
	displayName := ns.Object.GetAnnotations()["openshift.io/display-name"]
	nsArr := strings.Split(displayName, "/")

	return nsArr
}

// GetAncestor finds the nearest joint namespace of two subnamespaces in a hierarchy
func GetAncestor(sourceArr []string, destArr []string) (string, bool, error) {
	for i := len(sourceArr) - 1; i >= 0; i-- {
		for j := len(destArr) - 1; j >= 0; j-- {
			if sourceArr[i] == destArr[j] {
				// if we've reached the end of the loop, then the Ancestor is
				// the root namespace, and we return true - otherwise return false
				if (i == 0) && (j == 0) {
					return sourceArr[i], true, nil
				} else {
					return sourceArr[i], false, nil
				}
			}
		}
	}
	return "", false, fmt.Errorf("root namespace does not exist")
}
