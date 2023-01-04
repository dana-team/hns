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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MigrationHierarchySpec defines the desired state of MigrationHierarchy
type MigrationHierarchySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MigrationHierarchy. Edit MigrationHierarchy_types.go to remove/update
	CurrentNamespace string `json:"currentns"`
	ToNamespace      string `json:"tons"`
}

// MigrationHierarchyStatus defines the observed state of MigrationHierarchy
type MigrationHierarchyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase Phase `json:"phase,omitempty"`
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
