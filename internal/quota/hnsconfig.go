package quota

import (
	"context"
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"

	hnsv1 "github.com/dana-team/hns/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LimitRangeDefaults struct {
	Minimum        resourceQuantities `json:"minimum" yaml:"minimum"`
	DefaultRequest resourceQuantities `json:"defaultRequest" yaml:"defaultRequest"`
	DefaultLimit   resourceQuantities `json:"defaultLimit" yaml:"defaultLimit"`
	Maximum        resourceQuantities `json:"maximum" yaml:"maximum"`
	MinimumPVC     resourceQuantities `json:"minimumPVC" yaml:"minimumPVC"`
}

type resourceQuantities struct {
	Memory  resource.Quantity `json:"memory,omitempty" yaml:"memory,omitempty"`
	CPU     resource.Quantity `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Storage resource.Quantity `json:"storage,omitempty" yaml:"storage,omitempty"`
}

const (
	hnsConfigName = "hns-config"
)

// GetHnsConfigData retrieves the hnsConfig data from the cluster.
func GetHnsConfigData(ctx context.Context, k8sClient client.Client) (*hnsv1.HnsConfig, error) {
	hnsConfig := &hnsv1.HnsConfig{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: hnsConfigName, Namespace: danav1.HNSNamespace}, hnsConfig)
	if err != nil {
		return nil, err
	}

	return hnsConfig, nil
}

// GetObservedResources returns default values for all observed resources inside a ResourceQuotaSpec object.
// The observed resources are read from a hnsconfig object.
func GetObservedResources(ctx context.Context, k8sClient client.Client) (corev1.ResourceQuotaSpec, error) {
	resourcesConfig := &hnsv1.HnsConfig{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: QuotaConfig, Namespace: danav1.HNSNamespace}, resourcesConfig); err != nil {
		return corev1.ResourceQuotaSpec{}, fmt.Errorf("failed to get hnsconfig %q: %v", resourcesConfig.Name, err)
	}

	resources := corev1.ResourceList{}
	for _, resourceName := range resourcesConfig.Spec.ObservedDataResources.Resources {
		resources[corev1.ResourceName(resourceName)] = *ZeroDecimal
	}

	return corev1.ResourceQuotaSpec{Hard: resources}, nil
}
