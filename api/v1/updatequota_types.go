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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UpdatequotaSpec defines the desired state of Updatequota
type UpdatequotaSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ResourceQuotaSpec v1.ResourceQuotaSpec `json:"resourcequota"`
	DestNamespace     string               `json:"destns"`
	SourceNamespace   string               `json:"sourcens"`
}

// UpdatequotaStatus defines the observed state of Updatequota
type UpdatequotaStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Phase  Phase  `json:"phase,omitempty"`
	Reason string `json:"reason,omitempty"`
}

//+kubebuilder:object:root=true

// Updatequota is the Schema for the updatequota API
type Updatequota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpdatequotaSpec   `json:"spec,omitempty"`
	Status UpdatequotaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UpdatequotaList contains a list of Updatequota
type UpdatequotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Updatequota `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Updatequota{}, &UpdatequotaList{})
}
