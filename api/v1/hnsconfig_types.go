package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HNSConfigSpec defines the desired state of HNSConfig
type HNSConfigSpec struct {
	PermittedGroups   []string           `json:"permittedGroups"`
	ObservedResources []string           `json:"observedResources"`
	LimitRange        LimitRangeSettings `json:"limitRange"`
}

type LimitRangeSettings struct {
	Minimum        map[string]string `json:"minimum"`
	DefaultRequest map[string]string `json:"defaultRequest"`
	DefaultLimit   map[string]string `json:"defaultLimit"`
	Maximum        map[string]string `json:"maximum"`
	MinimumPVC     map[string]string `json:"minimumPVC"`
}

// HNSConfigStatus defines the observed state of HNSConfig
type HNSConfigStatus struct{}

//+kubebuilder:object:root=true

// HNSConfig is the Schema for the HNSConfigs API
type HNSConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HNSConfigSpec   `json:"spec,omitempty"`
	Status HNSConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type HNSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HNSConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HNSConfig{}, &HNSConfigList{})
}
