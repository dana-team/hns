package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HnsConfigSpec defines the desired state of HnsConfig
type HnsConfigSpec struct {
	PermittedGroups       PermittedGroups       `json:"permittedGroups"`
	ObservedDataResources ObservedDataResources `json:"observedDataResources"`
	LimitRangeDefaults    LimitRangeDefaults    `json:"limitRangeDefaults"`
}

type PermittedGroups struct {
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

type ObservedDataResources struct {
	Name      string   `json:"name"`
	Resources []string `json:"resources"`
}

type LimitRangeDefaults struct {
	Name       string             `json:"name"`
	Defaults   LimitRangeSettings `json:"defaults"`
	MinimumPVC map[string]string  `json:"minimumPVC"`
}

type LimitRangeSettings struct {
	Minimum        map[string]string `json:"minimum"`
	DefaultRequest map[string]string `json:"defaultRequest"`
	DefaultLimit   map[string]string `json:"defaultLimit"`
	Maximum        map[string]string `json:"maximum"`
}

// HnsConfigStatus defines the observed state of HnsConfig
type HnsConfigStatus struct{}

//+kubebuilder:object:root=true

// HnsConfig is the Schema for the HnsConfigs API
type HnsConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HnsConfigSpec   `json:"spec,omitempty"`
	Status HnsConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type HnsConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HnsConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HnsConfig{}, &HnsConfigList{})
}
