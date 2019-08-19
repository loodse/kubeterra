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

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/kubermatic/kubeterra/api/v1alpha1"
)

const (
	configurationFinalizerName = "configuration.finalizers.terraform.kubeterra.io"
	jobOwnerKey                = ".metadata.controller"
)

var (
	terraformv1alpha1GVStr = terraformv1alpha1.GroupVersion.String()
)

// TerraformConfigurationReconciler reconciles a TerraformConfiguration object
type TerraformConfigurationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformconfigurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformstates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformstates/status,verbs=get;update;patch

// logError returns closure which checks error != nil and call log.Error on it.
// error object will be returned without changes
func logError(log logr.Logger) func(error, string) error {
	return func(err error, msg string) error {
		if err != nil {
			log.Error(err, msg)
		}
		return err
	}
}

// SetupWithManager dependency inject controller
func (r *TerraformConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	idx := mgr.GetFieldIndexer()
	planIndexer := r.indexer("TerraformPlan")
	stateIndexer := r.indexer("TerraformState")

	if err := idx.IndexField(&terraformv1alpha1.TerraformPlan{}, jobOwnerKey, planIndexer); err != nil {
		return err
	}

	if err := idx.IndexField(&terraformv1alpha1.TerraformState{}, jobOwnerKey, stateIndexer); err != nil {
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
	var configObj terraformv1alpha1.TerraformConfiguration

	if err := r.Get(ctx, req.NamespacedName, &configObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return result, nil
		}
		log.Error(err, "unable to fetch TerraformConfiguration")
		return result, err
	}

	if ok, err := r.handleFinalizer(ctx, &configObj, r.deleteExternalResources); !ok {
		return result, errLogMsg(err, "finalizer handling failed")
	}

	if configObj.Spec.Paused {
		return result, nil
	}

	if configObj.Status.Phase == "" {
		configObj.Status.Phase = terraformv1alpha1.TerraformPhasePlanScheduled
		if err := errLogMsg(r.Status().Update(ctx, &configObj), "unable to update status"); err != nil {
			return result, err
		}
	}

	var stateObj = terraformv1alpha1.TerraformState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configObj.Name,
			Namespace: configObj.Namespace,
		},
	}

	stateObjKey, _ := client.ObjectKeyFromObject(&stateObj)
	err := r.Get(ctx, stateObjKey, &stateObj)
	switch {
	case apierrors.IsNotFound(err):
		if err2 := r.generateTerraformState(&configObj, &stateObj); err2 != nil {
			return result, errLogMsg(err2, "unable to generate TerraformState")
		}
		if err3 := r.Create(ctx, &stateObj); err3 != nil {
			return result, errLogMsg(err3, "unable to create TerraformState")
		}
	case err != nil:
		return result, errLogMsg(err, "unable to fetch TerraformState")
	}

	var planObj = terraformv1alpha1.TerraformPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configObj.Name,
			Namespace: configObj.Namespace,
		},
	}

	planObjKey, _ := client.ObjectKeyFromObject(&planObj)
	err = r.Get(ctx, planObjKey, &planObj)
	switch {
	case apierrors.IsNotFound(err):
		if err2 := r.generateTerraformPlan(&configObj, &planObj); err2 != nil {
			return result, errLogMsg(err2, "unable to generate TerraformPlan")
		}
		if err3 := r.Create(ctx, &planObj); err3 != nil {
			return result, errLogMsg(err3, "unable to create TerraformPlan")
		}
		if err4 := r.Status().Update(ctx, &planObj); err4 != nil {
			return result, errLogMsg(err4, "unable to update status of TerraformPlan")
		}
	case err != nil:
		return result, errLogMsg(err, "unable to fetch TerraformPlan")
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

func (r *TerraformConfigurationReconciler) indexer(kind string) func(runtime.Object) []string {
	return func(obj runtime.Object) []string {
		metaObj, ok := obj.(metav1.Object)
		if !ok {
			return nil
		}

		owner := metav1.GetControllerOf(metaObj)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != terraformv1alpha1GVStr || owner.Kind != kind {
			return nil
		}

		return []string{owner.Name}
	}
}

// handleFinalizer handles setup / removal of finalizer
// `ok == false` signalize to calling function to return
func (r *TerraformConfigurationReconciler) handleFinalizer(ctx context.Context,
	conf *terraformv1alpha1.TerraformConfiguration,
	cleanup func() error,
) (ok bool, err error) {

	if conf.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(conf.ObjectMeta.Finalizers, configurationFinalizerName) {
			conf.ObjectMeta.Finalizers = append(
				conf.ObjectMeta.Finalizers,
				configurationFinalizerName,
			)
			if err := r.Update(ctx, conf); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	}

	// TerraformConfiguration object is being deleted
	if containsString(conf.ObjectMeta.Finalizers, configurationFinalizerName) {
		if err := cleanup(); err != nil {
			return false, err
		}

		conf.ObjectMeta.Finalizers = removeString(conf.ObjectMeta.Finalizers, configurationFinalizerName)
		if err := r.Update(ctx, conf); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (r *TerraformConfigurationReconciler) deleteExternalResources() error {
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

type stateInfo struct {
	Version int    `json:"version"`
	Lineage string `json:"lineage"`
	Serial  int    `json:"serial"`
}
