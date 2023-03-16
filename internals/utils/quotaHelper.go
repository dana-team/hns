package utils

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"

	danav1 "github.com/dana-team/hns/api/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetSnsQuotaSpec(sns client.Object) corev1.ResourceQuotaSpec {
	if !isSns(sns) {
		return corev1.ResourceQuotaSpec{}
	}
	return sns.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
}

func GetSNSQuotaUsed(sns *ObjectContext) corev1.ResourceList {
	rqFlag, err := IsRq(sns, danav1.SelfOffset)
	if err != nil {
		sns.Log.Error(err, "unable to determine if sns is Rq")
	}
	if rqFlag {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: sns.Object.GetName(), Name: sns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			sns.Log.Error(err, "unable to get subnamespace quota object")
		}
		return quotaObj.Object.(*corev1.ResourceQuota).Status.Used
	} else {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: "", Name: sns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			sns.Log.Error(err, "unable to get subnamespace quota object")
		}
		return quotaObj.Object.(*quotav1.ClusterResourceQuota).Status.Total.Used
	}
}

func IsRqObject(sns client.Object) bool {
	if reflect.TypeOf(sns) == reflect.TypeOf(&corev1.ResourceQuota{}) {
		return true
	}
	return false
}

func IsSubspaceRqExists(sns *ObjectContext) bool {
	if !isSns(sns.Object) {
		return false
	}
	subspaceRqName := sns.Object.GetName()
	composeSubspaceResourceQuota := ComposeResourceQuota(subspaceRqName, subspaceRqName, danav1.Quotahard)

	subspaceResourceQuota, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: subspaceRqName, Namespace: subspaceRqName}, composeSubspaceResourceQuota)
	if err != nil {
		return false
	}

	if subspaceResourceQuota.IsPresent() {
		if !IsRqZeroed(subspaceResourceQuota.Object) {
			return true
		}
	}
	return false
}

func IsRqZeroed(rq client.Object) bool {
	if !IsRqObject(rq) {
		return false
	}
	for _, quantity := range GetRqSpec(rq).Hard {
		if quantity.Value() != 0 {
			return false
		}
	}
	return true
}

func GetRqSpec(rq client.Object) corev1.ResourceQuotaSpec {
	if !IsRqObject(rq) {
		return corev1.ResourceQuotaSpec{}
	}
	return rq.(*corev1.ResourceQuota).Spec
}

func GetRqUsed(rq client.Object) corev1.ResourceList {
	if !IsRqObject(rq) {
		return corev1.ResourceList{}
	}
	return rq.(*corev1.ResourceQuota).Status.Used
}

func GetSNSQuota(sns *ObjectContext) (corev1.ResourceList, error) {
	quotaParent := corev1.ResourceList{}
	//check by the annotation if the subnamespace have crq or rq, if it more than write in the annotation so it crq
	rqFlag, err := IsRq(sns, danav1.SelfOffset)
	if err != nil {
		return quotaParent, err
	}

	if rqFlag {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: "", Name: sns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			return quotaParent, err
		}
		return quotaObj.Object.(*corev1.ResourceQuota).Spec.Hard, nil
	} else {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: "", Name: sns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return quotaParent, err
		}
		return quotaObj.Object.(*quotav1.ClusterResourceQuota).Spec.Quota.Hard, nil
	}
}

func GetSNSQuotaObj(sns *ObjectContext) (*ObjectContext, error) {
	//check by the annotation if the subnamespace have crq or rq, if it more than write in the annotation so it crq
	rqFlag, err := IsRq(sns, danav1.SelfOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: sns.Object.GetName(), Name: sns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	} else {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: "", Name: sns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	}
}

func GetRootNSQuotaObj(sns *ObjectContext) (*ObjectContext, error) {
	quotaObj, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, client.ObjectKey{Namespace: sns.Object.GetName(), Name: sns.Object.GetName()}, &corev1.ResourceQuota{})
	if err != nil {
		return quotaObj, err
	}
	return quotaObj, nil
}

//func isCrq(sns client.Object) bool {
//	if reflect.TypeOf(sns) == reflect.TypeOf(&quotav1.ClusterResourceQuota{}) {
//		return true
//	}
//	return false
//}

func IsQuotaObjZeroed(QuotaObj client.Object) bool {
	for _, quantity := range GetQuotaObjSpec(QuotaObj).Hard {
		if quantity.Value() != 0 {
			return false
		}
	}
	return true
}

func IsSubspaceCrqExists(sns *ObjectContext) bool {
	if !isSns(sns.Object) {
		return false
	}
	snsCrq, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
	if err != nil {
		return false
	}

	if snsCrq.IsPresent() {
		if !IsQuotaObjZeroed(snsCrq.Object) {
			return true
		}
	}
	return false
}

func GetQuotaObjSpec(QuotaObj client.Object) corev1.ResourceQuotaSpec {
	crqCast, ok := QuotaObj.(*quotav1.ClusterResourceQuota)
	if !ok {
		return QuotaObj.(*corev1.ResourceQuota).Spec

	}
	return crqCast.Spec.Quota

}

func GetQuotaUsed(QuotaObj client.Object) corev1.ResourceList {
	crqCast, ok := QuotaObj.(*quotav1.ClusterResourceQuota)
	if !ok {
		return QuotaObj.(*corev1.ResourceQuota).Status.Used

	}
	return crqCast.Status.Total.Used
}

func GetSnsChildrenQuotaObjs(sns *ObjectContext) []*ObjectContext {
	var childrenQuotaObjs []*ObjectContext

	snsChildrenList, err := NewObjectContextList(sns.Ctx, sns.Log, sns.Client, &danav1.SubnamespaceList{}, client.InNamespace(sns.Object.GetName()))
	if err != nil {
		sns.Log.Error(err, "unable to get SubNamespace list")
	}

	rqFlag, err := IsRq(sns, danav1.ChildOffset)
	if err != nil {
		sns.Log.Error(err, "unable to get SubNamespace list")
	}

	for _, subns := range snsChildrenList.Objects.(*danav1.SubnamespaceList).Items {
		if rqFlag {
			childQuotaObj, _ := NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: subns.ObjectMeta.Name, Namespace: subns.ObjectMeta.Name}, &corev1.ResourceQuota{})
			childrenQuotaObjs = append(childrenQuotaObjs, childQuotaObj)
		} else {
			childQuotaObj, _ := NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: subns.ObjectMeta.Name}, &quotav1.ClusterResourceQuota{})
			childrenQuotaObjs = append(childrenQuotaObjs, childQuotaObj)
		}
	}

	return childrenQuotaObjs
}

func GetQuotaObjsListResources(quotaObjs []*ObjectContext) corev1.ResourceList {
	resourcesList := corev1.ResourceList{}

	for _, quotaObj := range quotaObjs {
		for quotaObjResource, quotaObjQuantity := range GetQuotaObjSpec(quotaObj.Object).Hard {
			addQuantityToResourceList(resourcesList, quotaObjResource, quotaObjQuantity)
		}
	}
	return resourcesList
}

func addQuantityToResourceList(resourceList corev1.ResourceList, resourceName corev1.ResourceName, quantity resource.Quantity) {
	if currentQuantity, ok := resourceList[resourceName]; ok {
		currentQuantity.Add(quantity)
		resourceList[resourceName] = currentQuantity
	} else {
		resourceList[resourceName] = quantity
	}
}

// GetCrqPointer gets a subnamespace and returns its crq pointer.
// If it has a crq (which means it's a subns or upper rp), it returns its name.
// If it doesn't, it returns the upper resourcepool's name.
func GetCrqPointer(subns client.Object) string {
	if subns.GetAnnotations()[danav1.IsRq] == danav1.True{
		return ""
	}
	if subns.GetLabels()[danav1.ResourcePool] == "false" ||
		subns.GetAnnotations()[danav1.IsUpperRp] == danav1.True {
		return subns.GetName()
	}
	return subns.GetAnnotations()[danav1.UpperRp]
 
 
 }
 