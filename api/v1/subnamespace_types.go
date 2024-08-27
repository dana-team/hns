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
	// Name is the name of the namespace that a Subnamespace is bound to
	Name string `json:"name,omitempty"`
}

type Namespaces struct {
	// Namespace is the name of a Subnamespace
	Namespace string `json:"namespace,omitempty"`

	// ResourceQuotaSpec represents the quota allocated to the Subnamespace
	ResourceQuotaSpec v1.ResourceQuotaSpec `json:"resourcequota,omitempty"`
}

type Total struct {
	// Allocated is a set of (resource name, quantity) pairs representing the total resources that
	// are allocated to the children Subnamespaces of a Subnamespace.
	Allocated v1.ResourceList `json:"allocated,omitempty"`

	// Free is a set of (resource name, quantity) pairs representing the total free/available/allocatable
	// resources that can still be allocated to the children Subnamespaces of a Subnamespace.
	Free v1.ResourceList `json:"free,omitempty"`
}

// SubnamespaceSpec defines the desired state of Subnamespace
type SubnamespaceSpec struct {
	// ResourceQuotaSpec represents the limitations that are associated with the Subnamespace.
	// This quota represents both the resources that can be allocated to children Subnamespaces
	// and the overall maximum quota consumption of the current Subnamespace and its children.
	ResourceQuotaSpec v1.ResourceQuotaSpec `json:"resourcequota,omitempty"`

	// The name of the namespace that this Subnamespace is bound to
	NamespaceRef namespaceRef `json:"namespaceRef,omitempty"`
}

// SubnamespaceStatus defines the observed state of Subnamespace
type SubnamespaceStatus struct {
	// Phase acts like a state machine for the Subnamespace.
	// It is a string and can be one of the following:
	// "" (Empty) - state for a Subnameapce that is being reconciled for the first time.
	// "Missing" - state for a Subnamespace that does not currently have a namespace bound to it
	// "Created" - state for a Subnamespace that exists and has a namespace bound to it and is being synced
	// "Migrated" - state for a Subnamespace that is currently undergoing migration to a different hierarchy
	Phase Phase `json:"phase,omitempty"`

	// Namespaces is an array of (name, ResourceQuotaSpec) pairs which are logically under the
	// Subnamespace in the hierarchy.
	Namespaces []Namespaces `json:"namespaces,omitempty"`

	// Total represents a summary of the resources allocated to children Subnamespaces
	// and the resources that are still free to allocate, from the total resources made
	// available in the ResourceQuotaSpec field in Spec
	Total Total `json:"total,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=sns

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
