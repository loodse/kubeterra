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

// TerraformProviderSpec defines the desired state of TerraformProvider
type TerraformProviderSpec struct {
	Provider   string                `json:"provider"`
	Parameters *runtime.RawExtension `json:"parameters"`
}

// TerraformProviderStatus defines the observed state of TerraformProvider
type TerraformProviderStatus struct{}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TerraformProvider is the Schema for the terraformproviders API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type TerraformProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerraformProviderSpec   `json:"spec,omitempty"`
	Status TerraformProviderStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TerraformProviderList contains a list of TerraformProvider
type TerraformProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerraformProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerraformProvider{}, &TerraformProviderList{})
}
