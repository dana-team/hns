package quota

import (
	danav1 "github.com/dana-team/hns/api/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsZeroed returns whether a quota object is zeroed.
func IsZeroed(QuotaObject client.Object) bool {
	for _, quantity := range GetQuotaObjectSpec(QuotaObject).Hard {
		if quantity.Value() != 0 {
			return false
		}
	}
	return true
}

// IsDefault returns whether a quota object is default.
func IsDefault(QuotaObject client.Object) bool {
	quotaSpec := GetQuotaObjectSpec(QuotaObject)

	for resourceName := range quotaSpec.Hard {
		if _, exists := DefaultQuotaHard[resourceName]; !exists {
			return false
		}
	}

	return true
}

// GetQuotaObjectSpec returns the quota of a quota object.
func GetQuotaObjectSpec(QuotaObject client.Object) corev1.ResourceQuotaSpec {
	crqCast, ok := QuotaObject.(*quotav1.ClusterResourceQuota)
	if !ok {
		return QuotaObject.(*corev1.ResourceQuota).Spec

	}
	return crqCast.Spec.Quota

}

// GetQuotaUsed returns the used value from the status of a quota object (RQ or CRQ).
func GetQuotaUsed(QuotaObject client.Object) corev1.ResourceList {
	crqCast, ok := QuotaObject.(*quotav1.ClusterResourceQuota)
	if !ok {
		return QuotaObject.(*corev1.ResourceQuota).Status.Used

	}
	return crqCast.Status.Total.Used
}

// GetCrqPointer gets a subnamespace and returns its crq-pointer. If it has a CRQ, which means it's a subnamesapce
// or an upper-rp, it returns its name. If it doesn't, it returns the upper resourcepool's name.
func GetCrqPointer(subns client.Object) string {
	if subns.GetAnnotations()[danav1.IsRq] == danav1.True {
		return subns.GetName()
	}
	if subns.GetLabels()[danav1.ResourcePool] == "false" ||
		subns.GetAnnotations()[danav1.IsUpperRp] == danav1.True {
		return subns.GetName()
	}
	return subns.GetAnnotations()[danav1.UpperRp]
}
