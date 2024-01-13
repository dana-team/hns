package quota

import (
	"github.com/dana-team/hns/internal/objectcontext"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterResourceQuota returns a ClusterResourceQuota object.
func ClusterResourceQuota(sns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	quotaObject, err := objectcontext.New(sns.Ctx, sns.Client, client.ObjectKey{Name: sns.Name()}, &quotav1.ClusterResourceQuota{})
	if err != nil {
		return quotaObject, err
	}

	return quotaObject, nil
}

// composeCRQ returns a ClusterResourceQuota object based on the given parameters.
func composeCRQ(name string, quota corev1.ResourceQuotaSpec, annSelector map[string]string) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: quotav1.ClusterResourceQuotaSpec{
			Selector: quotav1.ClusterResourceQuotaSelector{
				AnnotationSelector: annSelector,
			},
			Quota: quota,
		},
	}
}

// DoesSNSCRQExists returns true if a ClusterResourceQuota exists.
func DoesSNSCRQExists(sns *objectcontext.ObjectContext) (bool, *objectcontext.ObjectContext, error) {
	snsCrq, err := ClusterResourceQuota(sns)
	if err != nil {
		return false, nil, err
	}

	if snsCrq.IsPresent() {
		if !IsZeroed(snsCrq.Object) && !IsDefault(snsCrq.Object) {
			return true, snsCrq, nil
		}
	}

	return false, nil, nil
}
