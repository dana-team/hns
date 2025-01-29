/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdatequotaSpec defines the desired state of Updatequota
type UpdatequotaSpec struct {
	// ResourceQuotaSpec represents resources that need to be transferred
	// from one Subnamespace to another
	ResourceQuotaSpec v1.ResourceQuotaSpec `json:"resourcequota"`

	// DestNamespace is the name of the Subnamespace to which resources need to be transferred
	DestNamespace string `json:"destns"`

	// SourceNamespace is name of the Subnamespace from which resources need to be transferred
	SourceNamespace string `json:"sourcens"`
}

// UpdatequotaStatus defines the observed state of Updatequota
type UpdatequotaStatus struct {
	// Phase acts like a state machine for the Updatequota.
	// It is a string and can be one of the following:
	// "Error" - state for an Updatequota indicating that the operation could not be completed due to an error
	// "Complete" - state for an Updatequota indicating that the operation completed successfully
	Phase Phase `json:"phase,omitempty"`

	// Reason is a string explaining why an error occurred if it did; otherwise it’s empty
	Reason string `json:"reason,omitempty"`
}

// +kubebuilder:object:root=true

// Updatequota is the Schema for the updatequota API
type Updatequota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpdatequotaSpec   `json:"spec,omitempty"`
	Status UpdatequotaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UpdatequotaList contains a list of Updatequota
type UpdatequotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Updatequota `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Updatequota{}, &UpdatequotaList{})
}
