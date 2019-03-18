/*
Copyright 2019 The kubeterra Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// TerraformModuleSpec defines the desired state of TerraformModule
type TerraformModuleSpec struct {
	ProviderSelector *metav1.LabelSelector `json:"providerSelector"`
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector"`
}

// TerraformModuleStatus defines the observed state of TerraformModule
type TerraformModuleStatus struct {
	Conditions     []TerraformModuleCondition `json:"conditions,omitempty"`
	CurrentHash    string                     `json:"currentHash"`
	TerraformState *runtime.RawExtension      `json:"terraformState,omitempty"`
}

// TerraformModuleConditionType represents a condition type.
type TerraformModuleConditionType string

const (
	// TerraformModuleConditionReady represents that a given TerraformModuleCondition is in
	// ready state.
	TerraformModuleConditionReady TerraformModuleConditionType = "Ready"

	// TerraformModuleConditionFailed represents information about a final failure
	// that should not be retried.
	TerraformModuleConditionFailed TerraformModuleConditionType = "Failed"
)

// TerraformModuleCondition contains condition information for a TerraformModule.
type TerraformModuleCondition struct {
	// Type of the condition
	Type TerraformModuleConditionType `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string `json:"reason"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string `json:"message"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TerraformModule is the Schema for the terraformmodules API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type TerraformModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerraformModuleSpec   `json:"spec,omitempty"`
	Status TerraformModuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TerraformModuleList contains a list of TerraformModule
type TerraformModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerraformModule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerraformModule{}, &TerraformModuleList{})
}
