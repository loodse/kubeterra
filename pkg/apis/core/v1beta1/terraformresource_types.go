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

// TerraformResourceSpec defines the desired state of TerraformResource
type TerraformResourceSpec struct {
	Type       string                `json:"type"`
	Parameters *runtime.RawExtension `json:"parameters"`
}

// TerraformResourceStatus defines the observed state of TerraformResource
type TerraformResourceStatus struct {
	Conditions []TerraformResourceCondition `json:"conditions,omitempty"`
}

// TerraformResourceConditionType represents a condition type.
type TerraformResourceConditionType string

const (
	// TerraformResourceConditionReady represents that a given TerraformResourceCondition is in
	// ready state.
	TerraformResourceConditionReady TerraformResourceConditionType = "Ready"
)

// TerraformResourceCondition contains condition information for a TerraformResource.
type TerraformResourceCondition struct {
	// Type of the condition
	Type TerraformResourceConditionType `json:"type"`

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

// TerraformResource is the Schema for the terraformresources API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Module",type="string",JSONPath=".spec.module"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type TerraformResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerraformResourceSpec   `json:"spec,omitempty"`
	Status TerraformResourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TerraformResourceList contains a list of TerraformResource
type TerraformResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerraformResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerraformResource{}, &TerraformResourceList{})
}
