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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/kubermatic/kubeterra/api/v1alpha1"
)

// TerraformConfigurationReconciler reconciles a TerraformConfiguration object
type TerraformConfigurationReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	PodClient corev1typed.PodsGetter
}

// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformconfigurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformstates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformstates/status,verbs=get;update;patch

// Reconcile state
func (r *TerraformConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("terraformconfiguration", req.NamespacedName)

	return ctrl.Result{}, nil
}

// SetupWithManager dependency inject controller
func (r *TerraformConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.TerraformConfiguration{}).
		Owns(&terraformv1alpha1.TerraformPlan{}).
		Owns(&terraformv1alpha1.TerraformState{}).
		Complete(r)
}
