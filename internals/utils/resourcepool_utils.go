package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// IsNamespaceResourcePool returns a boolean value indicating if a namespace
// is a ResourcePool or not, based on its corresponding subnamespace
func IsNamespaceResourcePool(namespace *ObjectContext) (bool, error) {
	if !isNamespace(namespace.Object) {
		return false, nil
	}

	if IsRootNamespace(namespace.Object) {
		return false, nil
	}

	sns, err := GetSNSFromNamespace(namespace)
	if err != nil {
		return false, nil
	}

	snsResourcePoolLabel, err := IsSNSResourcePool(sns.Object.(*danav1.Subnamespace))
	if err != nil {
		return false, err
	}

	return snsResourcePoolLabel, nil
}

// GetSNSResourcePoolLabel returns the ResourcePool label of a subnamespace
func GetSNSResourcePoolLabel(sns client.Object) string {
	val, ok := sns.GetLabels()[danav1.ResourcePool]
	if !ok {
		return ""
	}

	return val
}

// IsSNSResourcePool returns a boolean value indicating if a subnamespace
// is a resource pool or not
func IsSNSResourcePool(sns client.Object) (bool, error) {
	snsResourcePoolLabel := GetSNSResourcePoolLabel(sns)
	if snsResourcePoolLabel == "" {
		return false, nil
	}

	isResourcePool, err := strconv.ParseBool(snsResourcePoolLabel)
	if err != nil {
		return false, err
	}
	return isResourcePool, nil
}

// GetSNSIsUpperResourcePoolAnnotation returns the is-upper-rp annotation of an SNS
func GetSNSIsUpperResourcePoolAnnotation(sns client.Object) string {
	val, ok := sns.GetAnnotations()[danav1.IsUpperRp]
	if !ok {
		return ""
	}
	return val
}

// IsSNSUpperResourcePoolFromAnnotation returns true if the subnamespace is an upper resource pool,
// based on its label
func IsSNSUpperResourcePoolFromAnnotation(sns client.Object) (bool, error) {
	snsUpperResourcePoolAnnotation := GetSNSIsUpperResourcePoolAnnotation(sns)
	if snsUpperResourcePoolAnnotation == "" {
		return false, nil
	}

	isUpperResourcePool, err := strconv.ParseBool(snsUpperResourcePoolAnnotation)
	if err != nil {
		return false, err
	}
	return isUpperResourcePool, nil
}

// IsChildUpperResourcePool gets a subnamespace father and child objects and returns whether the child should now become
// the upper resource pool
func IsChildUpperResourcePool(parentSNS, childSNS client.Object) (bool, error) {
	isParentSNSResourcePool, err := IsSNSResourcePool(parentSNS)
	if err != nil {
		return false, err
	}

	isChildSNSResourcePool, err := IsSNSResourcePool(childSNS)
	if err != nil {
		return false, err
	}

	isChildSNSUpperResourcePool, err := IsSNSUpperResourcePoolFromAnnotation(childSNS)
	if err != nil {
		return false, err
	}

	return !isParentSNSResourcePool && isChildSNSResourcePool && !isChildSNSUpperResourcePool, nil
}

// IsSNSUpperResourcePool returns true if the subnamespace is an upper resource pool,
// it happens only when the parent is from subnamespace kind or is a root namespace
func IsSNSUpperResourcePool(sns *ObjectContext) (bool, error) {
	parentName := sns.Object.GetNamespace()

	parentNS, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: parentName}, &corev1.Namespace{})
	if err != nil {
		return false, err
	}

	parentSNS, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: parentName, Namespace: GetNamespaceParent(parentNS.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return false, err
	}

	isSNSResourcePool, err := IsSNSResourcePool(sns.Object)
	if err != nil {
		return false, err
	}

	isParentResourcePool, err := IsSNSResourcePool(parentSNS.Object)
	if err != nil {
		return false, err
	}

	isParentRootNS := IsRootNamespace(parentNS.Object)

	if (isSNSResourcePool) && (isParentRootNS || !isParentResourcePool) {
		return true, nil
	}

	return false, nil
}

// IsNSUpperResourcePool returns true if the namespace is an upper resource pool
func IsNSUpperResourcePool(ns *ObjectContext) (bool, error) {
	sns, err := NewObjectContext(ns.Ctx, ns.Client, types.NamespacedName{Name: ns.Object.GetName(), Namespace: GetNamespaceParent(ns.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return false, err
	}

	return IsSNSUpperResourcePool(sns)
}
