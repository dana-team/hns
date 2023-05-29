package utils

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// GetSNSFromNamespace returns the subnamespace corresponding to a namespace
func GetSNSFromNamespace(namespace *ObjectContext) (*ObjectContext, error) {
	if !isNamespace(namespace.Object) {
		return nil, nil
	}

	if IsRootNamespace(namespace.Object) {
		return nil, nil
	}

	namespaceSns, err := NewObjectContext(namespace.Ctx, namespace.Client, types.NamespacedName{Name: namespace.Object.GetName(), Namespace: GetNamespaceParent(namespace.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return nil, err
	}

	return namespaceSns, nil
}

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
func DoesSNSNamespaceExist(sns *ObjectContext) (bool, error) {
	snsName := sns.Object.GetName()

	snsNS, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: snsName}, &corev1.Namespace{})
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

// GetSNSDepth returns the depth of an SNS by calculating its depth based on its parent
func GetSNSDepth(sns *ObjectContext) (string, error) {
	snsParentNSName := sns.Object.(*danav1.Subnamespace).GetNamespace()
	snsParentNamespace, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: snsParentNSName}, &corev1.Namespace{})
	if err != nil {
		return "", err
	}

	if !snsParentNamespace.IsPresent() {
		return "", fmt.Errorf("failed to find parent namespace '%s", snsParentNSName)
	}

	depth, _ := snsParentNamespace.Object.GetAnnotations()[danav1.Depth]
	depthInt, _ := strconv.Atoi(depth)
	depthInt = depthInt + 1

	return strconv.Itoa(depthInt), nil
}

// GetRqDepthFromSNS returns the rq-depth of the root namespace based on a given subnamesapce
func GetRqDepthFromSNS(sns *ObjectContext) (string, error) {
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
func GetAllChildren(obj *ObjectContext) []*ObjectContext {
	if obj == nil {
		return []*ObjectContext{}
	}

	children, err := NewObjectContextList(obj.Ctx, obj.Client, &danav1.SubnamespaceList{}, client.InNamespace(obj.Object.GetName()))
	if err != nil {
		return nil
	}

	var subspaceDescendant []*ObjectContext
	for _, objectItem := range children.Objects.(*danav1.SubnamespaceList).Items {
		var object *ObjectContext

		if isNamespace(obj.Object) {
			object, _ = NewObjectContext(obj.Ctx, obj.Client, types.NamespacedName{Name: objectItem.GetName()}, &corev1.Namespace{})
		} else {
			object, _ = NewObjectContext(obj.Ctx, obj.Client, types.NamespacedName{Name: objectItem.GetName(), Namespace: objectItem.GetNamespace()}, &danav1.Subnamespace{})
		}

		childrenObject := GetAllChildren(object)
		subspaceDescendant = append(subspaceDescendant, childrenObject...)
	}

	return append([]*ObjectContext{obj}, subspaceDescendant...)
}
