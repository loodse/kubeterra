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

// TerraformConfigurationTemplate defines some aspects of resulting Pod that will run terraform plan / teterraform apply
type TerraformConfigurationTemplate struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// Standard corev1 kubernetes API
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Standard corev1 kubernetes API
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// List of environment variables to set in the container.
	// Standard corev1 kubernetes API
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Pod volumes to mount into the container's filesystem.
	// Standard corev1 kubernetes API
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use in running terraform pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// Standard corev1 kubernetes API
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// TerraformPhase phase
// +kubebuilder:validation:Enum=PlanRunning;WaitingConfirmation;ApplyRunning;Done;Fail
type TerraformPhase string

// TerraformPhase ENUM
const (
	TerraformConfigurationPhasePlanRunning         TerraformPhase = "PlanRunning"
	TerraformConfigurationPhaseWaitingConfirmation TerraformPhase = "WaitingConfirmation"
	TerraformConfigurationPhaseApplyRunning        TerraformPhase = "ApplyRunning"
	TerraformConfigurationPhaseDone                TerraformPhase = "Done"
	TerraformConfigurationPhaseFail                TerraformPhase = "Fail"
)

// TerraformConfigurationSpec defines the desired state of TerraformConfiguration
type TerraformConfigurationSpec struct {
	// Indicates that the terraform apply should not happened.
	// +optional
	Paused bool `json:"paused"`

	// Indicates that the terrafor apply should happen without any further question.
	// +optional
	AutoApprove bool `json:"autoApprove"`

	// Configuration holds whole terraform configuration definition
	Configuration string `json:"configuration"`

	// Variable values, will be dumped to terraform.tfvars
	// +optional
	Values string `json:"values,omitempty"`

	// Defines some aspects of resulting Pod that will run terraform plan / teterraform apply
	// +optional
	Template *TerraformConfigurationTemplate `json:"template,omitempty"`
}

// TerraformConfigurationStatus defines the observed state of TerraformConfiguration
type TerraformConfigurationStatus struct {
	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Phase indicates current phase of the terraform action.
	// Is a enum PlanRunning;WaitingConfirmation;ApplyRunning;Done;Fail
	Phase TerraformPhase `json:"phase"`
}

// TerraformPlanSpec defines the desired state of TerraformPlan
type TerraformPlanSpec struct {
	// Configuration holds whole terraform configuration definition
	Configuration string `json:"configuration"`

	// Variable values, will be dumped to terraform.tfvars
	// +optional
	Values string `json:"values,omitempty"`

	// Defines some aspects of resulting Pod that will run terraform plan / teterraform apply
	// +optional
	Template *TerraformConfigurationTemplate `json:"template,omitempty"`
}

// TerraformPlanStatus defines the observed state of TerraformPlan
type TerraformPlanStatus struct {
	// Contain logs output
	// +optional
	Logs string `json:"logs"`

	// Base64 encoded contents of the `terraform plan -out`
	// +optional
	GeneratedPlan []byte `json:"generatedPlan,omitempty"`

	// Phase indicates current phase of the terraform action.
	// Is a enum PlanRunning;WaitingConfirmation;ApplyRunning;Done;Fail
	Phase TerraformPhase `json:"phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tfconfig;tfconfigs
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tfplan;tfplans

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

// TerraformStateSpec defines the desired state of TerraformState
type TerraformStateSpec struct {
	// Terraform State JSON object
	// +optional
	State runtime.RawExtension `json:"state,omitempty"`
}

// TerraformStateStatus defines the observed state of TerraformState
type TerraformStateStatus struct {
}

// +kubebuilder:object:root=true

// TerraformState is the Schema for the terraformstates API
type TerraformState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerraformStateSpec   `json:"spec,omitempty"`
	Status TerraformStateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TerraformStateList contains a list of TerraformState
type TerraformStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerraformState `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&TerraformConfiguration{},
		&TerraformConfigurationList{},
		&TerraformPlan{},
		&TerraformPlanList{},
		&TerraformState{},
		&TerraformStateList{},
	)
}
