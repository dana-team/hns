package upqutils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Compose returns an UpdateQuota object based on the given parameters.
func Compose(upqName, sourceNS, destNS, description string, resources corev1.ResourceQuotaSpec) *danav1.Updatequota {
	return &danav1.Updatequota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      upqName,
			Namespace: sourceNS,
			Annotations: map[string]string{
				danav1.Description: description,
			},
		},
		Spec: danav1.UpdatequotaSpec{
			ResourceQuotaSpec: resources,
			DestNamespace:     destNS,
			SourceNamespace:   sourceNS,
		},
	}
}
