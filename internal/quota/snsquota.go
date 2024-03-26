package quota

import (
	"fmt"
	"strconv"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SubnamespaceSpec returns the ResourceQuotaSpec of a subnamespace.
func SubnamespaceSpec(sns client.Object) corev1.ResourceQuotaSpec {
	return sns.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
}

// SubnamespaceObject returns the quota object of a subnamesapce.
func SubnamespaceObject(sns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	rqFlag, err := IsRQ(sns, danav1.SelfOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		return ResourceQuota(sns)
	}

	return ClusterResourceQuota(sns)
}

// EnsureSubnamespaceObject ensures that a quota object exists for a subnamespace.
func EnsureSubnamespaceObject(snsObject *objectcontext.ObjectContext, isRq bool) (ctrl.Result, error) {
	quotaObjectName := snsObject.Name()
	quotaSpec := SubnamespaceSpec(snsObject.Object)

	// if the subnamespace does not have a quota in its Spec, then set it to be equal to what it
	// currently uses, and create a quotaObject for it. This can happen when converting a ResourcePool to an SNS
	if len(quotaSpec.Hard) == 0 {
		quotaObject, err := setupObject(quotaObjectName, isRq, ZeroedQuota, snsObject)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := quotaObject.EnsureCreate(); err != nil {
			return ctrl.Result{}, err
		}

		// get the current used value from the quota object, so that we can later use the
		// value from its status to update the values in the spec of the quota object
		quotaObjectUsed, err := used(snsObject)
		if err != nil {
			return ctrl.Result{}, err
		}

		// if the value is nil then it means that it hasn't been created yet, so requeue
		if quotaObjectUsed == nil {
			return ctrl.Result{Requeue: true}, nil
		}

		if err := updateSubnamespaceSpec(snsObject, quotaObjectUsed); err != nil {
			return ctrl.Result{}, err
		}

		quotaSpec = corev1.ResourceQuotaSpec{Hard: quotaObjectUsed}
	}

	quotaObject, err := setupObject(quotaObjectName, isRq, quotaSpec, snsObject)
	if err != nil {
		return ctrl.Result{}, err
	}

	if IsZeroed(quotaObject.Object) {
		if err := UpdateObject(quotaObject, quotaSpec, isRq); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := quotaObject.EnsureCreate(); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// setupObject sets up - but doesn't create - a quota object with the given resources, the quotaObject can be either
// a ResourceQuota or a ClusterResourceQuota based on the depth of the subnamespace.
func setupObject(quotaObjName string, isRq bool, resources corev1.ResourceQuotaSpec, snsObject *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	var quotaObj *objectcontext.ObjectContext
	var err error

	if isRq {
		composedQuotaObj := composeRQ(quotaObjName, quotaObjName, resources)
		quotaObj, err = objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: quotaObjName, Namespace: quotaObjName}, composedQuotaObj)
		if err != nil {
			return quotaObj, err
		}
	} else {
		selector, err := subnamespaceDepth(snsObject)
		if err != nil {
			return quotaObj, err
		}
		crqMap := map[string]string{danav1.CrqSelector + "-" + selector: quotaObjName}
		composedQuotaObj := composeCRQ(quotaObjName, resources, crqMap)
		quotaObj, err = objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: quotaObjName}, composedQuotaObj)
		if err != nil {
			return quotaObj, err
		}
	}

	return quotaObj, nil
}

// used returns the Used value of the quota object.
func used(snsObject *objectcontext.ObjectContext) (corev1.ResourceList, error) {
	quotaObj, err := SubnamespaceObject(snsObject)
	if err != nil {
		return corev1.ResourceList{}, err
	}

	return GetQuotaUsed(quotaObj.Object), nil
}

// UpdateObject updates the spec of the quotaObject to be same as the given resources.
func UpdateObject(quotaObject *objectcontext.ObjectContext, resources corev1.ResourceQuotaSpec, isRq bool) error {
	if isRq {
		return quotaObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			log = log.WithValues("updated subnamespace", "ResourceQuotaSpec", "resources", resources)
			object.(*corev1.ResourceQuota).Spec = resources
			return object, log
		})
	} else {
		return quotaObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			log = log.WithValues("updated subnamespace", "ResourceQuotaSpecQuota", "resources", resources)
			object.(*quotav1.ClusterResourceQuota).Spec.Quota = resources
			return object, log
		})
	}
}

// updateSubnamespaceSpec updates the ResourceQuotaSpec of a subnamespace to be equal to resources.
func updateSubnamespaceSpec(snsObject *objectcontext.ObjectContext, resources corev1.ResourceList) error {
	return snsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated subnamespace", "ResourceQuotaSpecHard", "resources", resources)
		object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard = resources
		return object, log
	})
}

// subnamespaceDepth returns the depth of an SNS by calculating its depth based on its parent.
func subnamespaceDepth(sns *objectcontext.ObjectContext) (string, error) {
	snsParentNSName := sns.Object.(*danav1.Subnamespace).GetNamespace()
	snsParentNamespace, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: snsParentNSName}, &corev1.Namespace{})
	if err != nil {
		return "", err
	}

	if !snsParentNamespace.IsPresent() {
		return "", fmt.Errorf("failed to find parent namespace %q", snsParentNSName)
	}

	depth := snsParentNamespace.Object.GetAnnotations()[danav1.Depth]
	depthInt, err := strconv.Atoi(depth)
	if err != nil {
		return "", err
	}
	depthInt = depthInt + 1

	return strconv.Itoa(depthInt), nil
}

// SubnamespaceObjectFromAnnotation returns the quota object of a subnamesapce using annotations.
func SubnamespaceObjectFromAnnotation(sns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	rqFlag := sns.Object.GetAnnotations()[danav1.IsRq]

	if rqFlag == danav1.True {
		return ResourceQuota(sns)
	}

	return ClusterResourceQuota(sns)
}

// SubnamespaceParentObject returns the quota object of a subnamespace. The quota object can be either a
// ResourceQuota object or a ClusterResourceQuota object depending on  the depth in the hierarchy of the SNS.
func SubnamespaceParentObject(sns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	rqFlag, err := IsRQ(sns, danav1.ParentOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		quotaObj, err := objectcontext.New(sns.Ctx, sns.Client, client.ObjectKey{Namespace: sns.Object.GetNamespace(), Name: sns.Object.GetNamespace()}, &corev1.ResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	} else {
		quotaObj, err := objectcontext.New(sns.Ctx, sns.Client, client.ObjectKey{Name: sns.Object.GetNamespace()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	}
}

// SubnamespaceSiblingObjects returns a slice of the quota objects of all the siblings of a subnamespace.
func SubnamespaceSiblingObjects(sns *objectcontext.ObjectContext) []*objectcontext.ObjectContext {
	var siblings []*objectcontext.ObjectContext

	namespaceList, err := objectcontext.NewList(sns.Ctx, sns.Client, &corev1.NamespaceList{}, client.MatchingLabels{danav1.Parent: sns.Object.GetNamespace()})
	if err != nil {
		sns.Log.Error(err, "unable to get namespace list")
	}

	rqFlag, err := IsRQ(sns, danav1.SelfOffset)
	if err != nil {
		sns.Log.Error(err, "unable to determine if sns is Rq")
	}

	for _, namespace := range namespaceList.Objects.(*corev1.NamespaceList).Items {
		if rqFlag {
			siblingQuotaObj, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: namespace.ObjectMeta.Name, Namespace: namespace.ObjectMeta.Name}, &corev1.ResourceQuota{})
			if err != nil {
				sns.Log.Error(err, "unable to get RQ object")
			}
			siblings = append(siblings, siblingQuotaObj)
		} else {
			siblingQuotaObj, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: namespace.ObjectMeta.Name}, &quotav1.ClusterResourceQuota{})
			if err != nil {
				sns.Log.Error(err, "unable to get CRQ object")
			}
			siblings = append(siblings, siblingQuotaObj)
		}
	}

	return siblings
}

// DoesSubnamespaceObjectExist returns true if an object quota exists.
func DoesSubnamespaceObjectExist(sns *objectcontext.ObjectContext) (bool, *objectcontext.ObjectContext, error) {
	quotaObj, err := SubnamespaceObject(sns)
	if err != nil {
		return false, nil, err
	}
	if quotaObj.IsPresent() {
		if !IsZeroed(quotaObj.Object) {
			return true, quotaObj, nil
		}
	}

	return false, nil, nil
}

// SubnamespaceChildrenObjects returns the quota objects of all the children of a subnamespace.
func SubnamespaceChildrenObjects(sns *objectcontext.ObjectContext) []*objectcontext.ObjectContext {
	var childrenQuotaObjects []*objectcontext.ObjectContext

	snsChildrenList, err := objectcontext.NewList(sns.Ctx, sns.Client, &danav1.SubnamespaceList{}, client.InNamespace(sns.Name()))
	if err != nil {
		sns.Log.Error(err, "unable to get SubNamespace list")
	}

	rqFlag, err := IsRQ(sns, danav1.ChildOffset)
	if err != nil {
		sns.Log.Error(err, "unable to get SubNamespace list")
	}

	for _, subns := range snsChildrenList.Objects.(*danav1.SubnamespaceList).Items {
		if rqFlag {
			childQuotaObj, _ := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: subns.ObjectMeta.Name, Namespace: subns.ObjectMeta.Name}, &corev1.ResourceQuota{})
			childrenQuotaObjects = append(childrenQuotaObjects, childQuotaObj)
		} else {
			childQuotaObj, _ := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: subns.ObjectMeta.Name}, &quotav1.ClusterResourceQuota{})
			childrenQuotaObjects = append(childrenQuotaObjects, childQuotaObj)
		}
	}

	return childrenQuotaObjects
}
