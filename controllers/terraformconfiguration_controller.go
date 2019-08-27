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
	"encoding/json"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/loodse/kubeterra/api/v1alpha1"
)

const (
	configurationFinalizerName = "configuration.finalizers.terraform.kubeterra.io"
)

// TerraformConfigurationReconciler reconciles a TerraformConfiguration object
type TerraformConfigurationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformconfigurations,verbs=*
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformconfigurations/status,verbs=*
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans,verbs=*
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans/status,verbs=*
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformstates,verbs=*
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformstates/status,verbs=*

// SetupWithManager dependency inject controller
func (r *TerraformConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mgrIndexer := mgr.GetFieldIndexer()
	indexer := indexerFunc("TerraformConfiguration", terraformv1alpha1.GroupVersion.String())

	if err := mgrIndexer.IndexField(&terraformv1alpha1.TerraformPlan{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	if err := mgrIndexer.IndexField(&terraformv1alpha1.TerraformState{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.TerraformConfiguration{}).
		Owns(&terraformv1alpha1.TerraformPlan{}).
		Owns(&terraformv1alpha1.TerraformState{}).
		Complete(r)
}

// Reconcile state
func (r *TerraformConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("terraformconfiguration", req.NamespacedName)
	errLogMsg := logError(log)

	result := ctrl.Result{}
	var tfconfig terraformv1alpha1.TerraformConfiguration

	if err := r.Get(ctx, req.NamespacedName, &tfconfig); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("not found")
			return result, nil
		}
		return result, errLogMsg(err, "unable to fetch TerraformConfiguration")
	}

	if ok, err := r.handleFinalizer(ctx, &tfconfig, r.deleteExternalResources); !ok {
		return result, errLogMsg(err, "finalizer handling failed")
	}

	if tfconfig.Spec.Paused {
		log.Info("paused")
		return result, nil
	}

	if tfconfig.Status.Phase == "" {
		tfconfig.Status.Phase = terraformv1alpha1.TerraformPhasePlanScheduled
		if err := errLogMsg(r.Status().Update(ctx, &tfconfig), "unable to update status"); err != nil {
			return result, err
		}
	}

	var tfstate = terraformv1alpha1.TerraformState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tfconfig.Name,
			Namespace: tfconfig.Namespace,
		},
	}

	tfstateKey, _ := client.ObjectKeyFromObject(&tfstate)
	err := r.Get(ctx, tfstateKey, &tfstate)
	switch {
	case apierrors.IsNotFound(err):
		if err2 := r.generateTerraformState(&tfconfig, &tfstate); err2 != nil {
			return result, errLogMsg(err2, "unable to generate TerraformState")
		}
		if err3 := r.Create(ctx, &tfstate); err3 != nil {
			return result, errLogMsg(err3, "unable to create TerraformState")
		}
	case err != nil:
		return result, errLogMsg(err, "unable to fetch TerraformState")
	}

	var tfplan = terraformv1alpha1.TerraformPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tfconfig.Name,
			Namespace: tfconfig.Namespace,
		},
	}

	tfplanKey, _ := client.ObjectKeyFromObject(&tfplan)
	err = r.Get(ctx, tfplanKey, &tfplan)
	switch {
	case apierrors.IsNotFound(err):
		if err2 := r.generateTerraformPlan(&tfconfig, &tfplan); err2 != nil {
			return result, errLogMsg(err2, "unable to generate TerraformPlan")
		}
		if err3 := r.Create(ctx, &tfplan); err3 != nil {
			return result, errLogMsg(err3, "unable to create TerraformPlan")
		}
		if err4 := r.Status().Update(ctx, &tfplan); err4 != nil {
			return result, errLogMsg(err4, "unable to update status of TerraformPlan")
		}
	case err != nil:
		return result, errLogMsg(err, "unable to fetch TerraformPlan")
	}

	var tfplanNew terraformv1alpha1.TerraformPlan
	if err := r.generateTerraformPlan(&tfconfig, &tfplanNew); err != nil {
		return result, errLogMsg(err, "unable to generate new TerraformPlan")
	}

	if !reflect.DeepEqual(&tfplanNew.Spec, &tfplan.Spec) {
		tfplan.ObjectMeta.DeepCopyInto(&tfplanNew.ObjectMeta)
		if err := r.Update(ctx, &tfplanNew); err != nil {
			return result, errLogMsg(err, "unable to update TerraformPlan")
		}
	}

	return result, nil
}

func (r *TerraformConfigurationReconciler) generateTerraformState(
	config *terraformv1alpha1.TerraformConfiguration,
	state *terraformv1alpha1.TerraformState,
) error {

	lineage, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	initialState := stateInfo{
		Version: 4,
		Serial:  1,
		Lineage: lineage,
	}
	initialStateMarshaled, err := json.Marshal(initialState)
	if err != nil {
		return err
	}

	state.Spec.State = &runtime.RawExtension{
		Raw: initialStateMarshaled,
	}

	if err := ctrl.SetControllerReference(config, state, r.Scheme); err != nil {
		return err
	}

	return nil
}

func (r *TerraformConfigurationReconciler) generateTerraformPlan(
	config *terraformv1alpha1.TerraformConfiguration,
	plan *terraformv1alpha1.TerraformPlan,
) error {

	plan.Spec.Approved = config.Spec.AutoApprove
	plan.Spec.Configuration = config.Spec.Configuration
	plan.Spec.Values = config.Spec.Values
	plan.Spec.Template = config.Spec.Template.DeepCopy()
	plan.Status.Phase = terraformv1alpha1.TerraformPhasePlanScheduled

	if err := ctrl.SetControllerReference(config, plan, r.Scheme); err != nil {
		return err
	}

	return nil
}

// handleFinalizer handles setup / removal of finalizer
// `ok == false` signalize to calling function to return
func (r *TerraformConfigurationReconciler) handleFinalizer(ctx context.Context,
	tfconf *terraformv1alpha1.TerraformConfiguration,
	cleanup func() error,
) (ok bool, err error) {

	if tfconf.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(tfconf.ObjectMeta.Finalizers, configurationFinalizerName) {
			tfconf.ObjectMeta.Finalizers = append(
				tfconf.ObjectMeta.Finalizers,
				configurationFinalizerName,
			)
			if err := r.Update(ctx, tfconf); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	}

	// TerraformConfiguration object is being deleted
	if containsString(tfconf.ObjectMeta.Finalizers, configurationFinalizerName) {
		if err := cleanup(); err != nil {
			return false, err
		}

		tfconf.ObjectMeta.Finalizers = removeString(tfconf.ObjectMeta.Finalizers, configurationFinalizerName)
		if err := r.Update(ctx, tfconf); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (r *TerraformConfigurationReconciler) deleteExternalResources() error {
	// TODO
	return nil
}
