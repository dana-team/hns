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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigrationHierarchySpec defines the desired state of MigrationHierarchy
type MigrationHierarchySpec struct {
	// CurrentNamespace is name of the Subnamespace that is being migrated
	CurrentNamespace string `json:"currentns"`

	// ToNamespace is the name of the Subnamespace that represents the new parent 
	// of the Subnamespace that needs to be migrated
	ToNamespace      string `json:"tons"`
}

// MigrationHierarchyStatus defines the observed state of MigrationHierarchy
type MigrationHierarchyStatus struct {
	// Phase acts like a state machine for the Migrationhierarchy. 
	// It is a string and can be one of the following:
	// "Error" - state for a Migrationhierarchy indicating that the operation could not be completed due to an error
	// "Complete" - state for a Migrationhierarchy indicating that the operation completed successfully
	Phase Phase `json:"phase,omitempty"`

	// Reason is a string explaining why an error occurred if it did; otherwise it’s empty
	Reason string `json:"reason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// MigrationHierarchy is the Schema for the migrationhierarchies API
type MigrationHierarchy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationHierarchySpec   `json:"spec,omitempty"`
	Status MigrationHierarchyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationHierarchyList contains a list of MigrationHierarchy
type MigrationHierarchyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationHierarchy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationHierarchy{}, &MigrationHierarchyList{})
}
