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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TerraformPlanPhase phase
// +kubebuilder:validation:Enum=Scheduled;Running;Done;Failed
type TerraformPlanPhase string

// TerraformPlanPhase ENUM
const (
	TerraformPlanPhaseScheduled TerraformPlanPhase = "Scheduled"
	TerraformPlanPhaseRunning   TerraformPlanPhase = "Running"
	TerraformPlanPhaseDone      TerraformPlanPhase = "Done"
	TerraformPlanPhaseFailed    TerraformPlanPhase = "Failed"
)

// TerraformPlanSpec defines the desired state of TerraformPlan
type TerraformPlanSpec struct {
}

// TerraformPlanStatus defines the observed state of TerraformPlan
type TerraformPlanStatus struct {
	// Phase indicates current phase of the terraform action.
	// Is a enum Scheduled;Running;Done;Failed
	Phase TerraformPlanPhase `json:"phase"`

	// Contain logs output
	// +optional
	Logs string `json:"logs"`

	// Base64 encoded contents of the `terraform plan -out`
	// +optional
	GeneratedPlan []byte `json:"generatedPlan,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tfplan;tfplans
// +kubebuilder:printcolumn:name=Phase,type=string,JSONPath=`.status.phase`

// TerraformPlan is the Schema for the terraformplans API
type TerraformPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerraformPlanSpec   `json:"spec,omitempty"`
	Status TerraformPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TerraformPlanList contains a list of TerraformPlan
type TerraformPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerraformPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerraformPlan{}, &TerraformPlanList{})
}
