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

type namespaceRef struct {
	Name string `json:"name,omitempty"`
}

type Namespaces struct {
	Namespace         string               `json:"namespace,omitempty"`
	ResourceQuotaSpec v1.ResourceQuotaSpec `json:"resourcequota,omitempty"`
}

type Total struct {
	Allocated v1.ResourceList `json:"allocated,omitempty"`
	Free      v1.ResourceList `json:"free,omitempty"`
}

// SubnamespaceSpec defines the desired state of Subnamespace
type SubnamespaceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ResourceQuotaSpec v1.ResourceQuotaSpec `json:"resourcequota,omitempty"`
	NamespaceRef      namespaceRef         `json:"namespaceRef,omitempty"`
}

// SubnamespaceStatus defines the observed state of Subnamespace
type SubnamespaceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regeneraste code after modifying this file
	Phase      Phase        `json:"phase,omitempty"`
	Namespaces []Namespaces `json:"namespaces,omitempty"`
	Total      Total        `json:"total,omitempty"`
}

// +kubebuilder:object:root=true

// Subnamespace is the Schema for the subnamespaces API
type Subnamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnamespaceSpec   `json:"spec,omitempty"`
	Status SubnamespaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubnamespaceList contains a list of Subnamespace
type SubnamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnamespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Subnamespace{}, &SubnamespaceList{})
}
