package resourcepool

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// IsNSResourcePool returns a boolean value indicating if a namespace
// is a ResourcePool or not, based on its corresponding subnamespace
func IsNSResourcePool(namespace *objectcontext.ObjectContext) (bool, error) {
	sns, err := nsutils.SNSFromNamespace(namespace)
	if err != nil {
		return false, nil
	}

	if sns == nil {
		return false, nil
	}

	snsResourcePoolLabel, err := IsSNSResourcePool(sns.Object.(*danav1.Subnamespace))
	if err != nil {
		return false, err
	}

	return snsResourcePoolLabel, nil
}

// IsSNSResourcePool returns a boolean value indicating if a subnamespace
// is a resource pool or not
func IsSNSResourcePool(sns client.Object) (bool, error) {
	snsResourcePoolLabel := SNSLabel(sns)
	if snsResourcePoolLabel == "" {
		return false, nil
	}

	isResourcePool, err := strconv.ParseBool(snsResourcePoolLabel)
	if err != nil {
		return false, err
	}
	return isResourcePool, nil
}

// SNSLabel returns the ResourcePool label of a subnamespace
func SNSLabel(sns client.Object) string {
	val, ok := sns.GetLabels()[danav1.ResourcePool]
	if !ok {
		return ""
	}

	return val
}

// SNSIsUpperAnnotation returns the is-upper-rp annotation of an SNS
func SNSIsUpperAnnotation(sns client.Object) string {
	val, ok := sns.GetAnnotations()[danav1.IsUpperRp]
	if !ok {
		return ""
	}
	return val
}

// IsSNSUpperFromAnnotation returns true if the subnamespace is an upper resource pool,
// based on its label
func IsSNSUpperFromAnnotation(sns client.Object) (bool, error) {
	snsUpperResourcePoolAnnotation := SNSIsUpperAnnotation(sns)
	if snsUpperResourcePoolAnnotation == "" {
		return false, nil
	}

	isUpperResourcePool, err := strconv.ParseBool(snsUpperResourcePoolAnnotation)
	if err != nil {
		return false, err
	}
	return isUpperResourcePool, nil
}

// IsChildUpper gets a subnamespace father and child objects and returns whether the child should now become
// the upper resource pool
func IsChildUpper(parentSNS, childSNS client.Object) (bool, error) {
	isParentSNSResourcePool, err := IsSNSResourcePool(parentSNS)
	if err != nil {
		return false, err
	}

	isChildSNSResourcePool, err := IsSNSResourcePool(childSNS)
	if err != nil {
		return false, err
	}

	isChildSNSUpperResourcePool, err := IsSNSUpperFromAnnotation(childSNS)
	if err != nil {
		return false, err
	}

	return !isParentSNSResourcePool && isChildSNSResourcePool && !isChildSNSUpperResourcePool, nil
}

// IsSNSUpper returns true if the subnamespace is an upper resource pool,
// it happens only when the parent is from subnamespace kind or is a root namespace
func IsSNSUpper(sns *objectcontext.ObjectContext) (bool, error) {
	parentName := sns.Namespace()

	parentNS, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: parentName}, &corev1.Namespace{})
	if err != nil {
		return false, err
	}

	parentSNS, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: parentName, Namespace: nsutils.Parent(parentNS.Object)}, &danav1.Subnamespace{})
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

	isParentRootNS := nsutils.IsRoot(parentNS.Object)

	if (isSNSResourcePool) && (isParentRootNS || !isParentResourcePool) {
		return true, nil
	}

	return false, nil
}

// IsNSUpperResourcePool returns true if the namespace is an upper resource pool
func IsNSUpperResourcePool(ns *objectcontext.ObjectContext) (bool, error) {
	sns, err := objectcontext.New(ns.Ctx, ns.Client, types.NamespacedName{Name: ns.Name(), Namespace: nsutils.Parent(ns.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return false, err
	}

	return IsSNSUpper(sns)
}

// SetSNSResourcePoolLabel sets a ResourcePool label on the subnamespace based
// on the ResourcePool label in its parent namespace
func SetSNSResourcePoolLabel(snsParentNS, snsObject *objectcontext.ObjectContext) error {
	isResourcePool, err := IsNSResourcePool(snsParentNS)
	if err != nil {
		return err
	}

	if err := snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues(danav1.ResourcePool, isResourcePool)
		object.SetLabels(map[string]string{danav1.ResourcePool: strconv.FormatBool(isResourcePool)})
		return object, log
	}); err != nil {
		return err
	}

	return nil

}
