package snsutils

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetNamespaceSNSPointerAnnotation returns the sns-pointer annotation of a namespace
func GetNamespaceSNSPointerAnnotation(namespace client.Object) string {
	return namespace.GetAnnotations()[danav1.SnsPointer]
}

// GetAncestor finds the nearest joint namespace of two subnamespaces in a hierarchy
func GetAncestor(source []string, dest []string) (string, bool, error) {
	for i := len(source) - 1; i >= 0; i-- {
		for j := len(dest) - 1; j >= 0; j-- {
			if source[i] == dest[j] {
				if (i == 0) && (j == 0) {
					return source[i], true, nil
				} else {
					return source[i], false, nil
				}
			}
		}
	}
	return "", false, fmt.Errorf("failed to find ancestor namespace")
}

// DoesSNSNamespaceExist checks whether a namespace of a given name exists already in the cluster
func DoesSNSNamespaceExist(sns *objectcontext.ObjectContext) (bool, error) {
	snsName := sns.Name()

	snsNS, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: snsName}, &corev1.Namespace{})
	if err != nil {
		return false, err
	}

	// if the phase of the subnamespace is Migrated then it's fine that it already exists
	if snsNS.IsPresent() {
		ph := sns.Object.(*danav1.Subnamespace).Status.Phase
		if ph != danav1.Migrated {
			return true, nil
		}
	}
	return false, nil
}

// GetRqDepthFromSNS returns the rq-depth of the root namespace based on a given subnamesapce
func GetRqDepthFromSNS(sns *objectcontext.ObjectContext) (string, error) {
	rootNS := corev1.Namespace{}
	parentNS := corev1.Namespace{}
	if err := sns.Client.Get(sns.Ctx, types.NamespacedName{Name: sns.Object.GetNamespace()}, &parentNS); err != nil {
		return "", err
	}
	if err := sns.Client.Get(sns.Ctx, types.NamespacedName{Name: parentNS.Annotations[danav1.RootCrqSelector]}, &rootNS); err != nil {
		return "", err
	}
	return rootNS.Annotations[danav1.RqDepth], nil
}

// GetAllChildren returns a slice of all the descendants of a namespace or subnamespace, including it itself
func GetAllChildren(obj *objectcontext.ObjectContext) []*objectcontext.ObjectContext {
	if obj == nil {
		return []*objectcontext.ObjectContext{}
	}

	children, err := objectcontext.NewList(obj.Ctx, obj.Client, &danav1.SubnamespaceList{}, client.InNamespace(obj.Name()))
	if err != nil {
		return nil
	}

	var subspaceDescendant []*objectcontext.ObjectContext
	for _, objectItem := range children.Objects.(*danav1.SubnamespaceList).Items {
		var object *objectcontext.ObjectContext

		if nsutils.IsNamespace(obj.Object) {
			object, _ = objectcontext.New(obj.Ctx, obj.Client, types.NamespacedName{Name: objectItem.GetName()}, &corev1.Namespace{})
		} else {
			object, _ = objectcontext.New(obj.Ctx, obj.Client, types.NamespacedName{Name: objectItem.GetName(), Namespace: objectItem.GetNamespace()}, &danav1.Subnamespace{})
		}

		childrenObject := GetAllChildren(object)
		subspaceDescendant = append(subspaceDescendant, childrenObject...)
	}

	return append([]*objectcontext.ObjectContext{obj}, subspaceDescendant...)
}
