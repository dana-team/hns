package quota

import (
	"context"
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapData struct {
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
	configMapName = "hns-config"
)

// getConfigMapData retrieves the ConfigMap data from the cluster.
func getConfigMapData(ctx context.Context, k8sClient client.Client, key string) (string, error) {
	cm := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: danav1.HNSNamespace}, cm)
	if err != nil {
		return "", err
	}
	marshalledData, ok := cm.Data[key]
	if !ok {
		return "", fmt.Errorf("configmap %q does not contain key %s", configMapName, limitConfigMapKey)
	}

	return marshalledData, nil
}

// parseLimitRangeData parses the data from the ConfigMap into a ConfigMapData struct.
func parseLimitRangeData(data string) (ConfigMapData, error) {
	limitRangeData := ConfigMapData{}
	if err := yaml.Unmarshal([]byte(data), &limitRangeData); err != nil {
		return ConfigMapData{}, fmt.Errorf("failed parsing limit range data: %w", err)
	}
	return limitRangeData, nil
}
