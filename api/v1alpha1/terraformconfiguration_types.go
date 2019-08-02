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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TerraformConfigurationSpec defines the desired state of TerraformConfiguration
type TerraformConfigurationSpec struct {
	// Configuration holds whole terraform configuration definition
	Configuration string `json:"configuration"`

	// Indicates that the deployment is paused.
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// TerraformConfigurationStatus defines the observed state of TerraformConfiguration
type TerraformConfigurationStatus struct {
	Outputs runtime.RawExtension `json:"outputs"`

	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Information when was the last time the job was successfully scheduled.
	// +optional
	LastApplyTime *metav1.Time `json:"lastApplyTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tfconfig

// TerraformConfiguration is the Schema for the terraformconfigurations API
type TerraformConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerraformConfigurationSpec   `json:"spec,omitempty"`
	Status TerraformConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TerraformConfigurationList contains a list of TerraformConfiguration
type TerraformConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerraformConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerraformConfiguration{}, &TerraformConfigurationList{})
}
