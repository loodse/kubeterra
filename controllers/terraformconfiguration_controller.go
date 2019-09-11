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
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terapi "github.com/loodse/kubeterra/api/v1alpha1"
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
	indexer := indexerFunc("TerraformConfiguration", terapi.GroupVersion.String())

	if err := mgrIndexer.IndexField(&terapi.TerraformPlan{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	if err := mgrIndexer.IndexField(&terapi.TerraformState{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terapi.TerraformConfiguration{}).
		Complete(r)
}

// Reconcile state
func (r *TerraformConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("terraformconfiguration", req.NamespacedName)
	errLogMsg := logError(log)
	defer log.Info("done")

	var tfconfig terapi.TerraformConfiguration

	log.Info("find TerraformConfiguration")
	if err := r.Get(ctx, req.NamespacedName, &tfconfig); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("TerraformConfiguration not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errLogMsg(err, "unable to get TerraformConfiguration")
	}

	log.Info("handle finalizers on TerraformConfiguration")
	if ok, err := r.handleFinalizers(ctx, &tfconfig, r.deleteExternalResources); !ok {
		return ctrl.Result{}, errLogMsg(err, "finalizer handling failed")
	}

	if tfconfig.Spec.Paused {
		log.Info("TerraformConfiguration is paused")
		return ctrl.Result{}, nil
	}

	if tfconfig.Status.Phase == "" {
		tfconfig.Status.Phase = terapi.TerraformPhasePlanScheduled
		return ctrl.Result{}, errLogMsg(r.Status().Update(ctx, &tfconfig), "unable to update TerraformConfiguration.Status")
	}

	var (
		tfstate = terapi.TerraformState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tfconfig.Name,
				Namespace: tfconfig.Namespace,
			},
		}
		tfplan = terapi.TerraformPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tfconfig.Name,
				Namespace: tfconfig.Namespace,
			},
		}
		newState = func() error { return r.generateTerraformState(&tfconfig, &tfstate) }
		newPlan  = func() error { return generateTerraformPlan(&tfconfig, &tfplan, r.Scheme) }
	)

	log.Info("get TerraformState")
	created, err := findOrCreate(ctx, r.Client, &tfstate, newState)
	if err != nil {
		return ctrl.Result{}, errLogMsg(err, "unable to get TerraformState")
	}
	if created {
		log.Info("TerraformState created")
	}

	log.Info("get TerraformPlan")
	created, err = findOrCreate(ctx, r.Client, &tfplan, newPlan)
	if err != nil {
		return ctrl.Result{}, errLogMsg(err, "unable to get TerraformPlan")
	}

	if created {
		log.Info("TerraformPlan created")
	}

	if tfconfig.Status.Phase != tfplan.Status.Phase {
		log.Info("TerraformConfiguration.Status update")
		if statusErr := r.Status().Update(ctx, &tfconfig); statusErr != nil {
			return ctrl.Result{}, errLogMsg(statusErr, "unable to update TerraformConfiguration.Status")
		}
	}

	result := ctrl.Result{}

	if tfconfig.Spec.RepeatEvery != nil {
		var requeueAfter time.Duration
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if _, err = findOrCreate(ctx, r.Client, &tfplan, noopGenerator); err != nil {
				return err
			}
			requeueAfter = planScheduleNextRunAt(&tfconfig, &tfplan)
			return r.Update(ctx, &tfplan)
		})
		if retryErr != nil {
			return ctrl.Result{}, errLogMsg(retryErr, "unable to update TerraformPlan.Spec.NextRunAt")
		}
		result.RequeueAfter = requeueAfter
		log.Info("RequeueAfter", "RequeueAfter", result.RequeueAfter)
	}

	return result, nil
}

func (r *TerraformConfigurationReconciler) generateTerraformState(config *terapi.TerraformConfiguration, state *terapi.TerraformState) error {
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

	return ctrl.SetControllerReference(config, state, r.Scheme)
}

func planScheduleNextRunAt(tfconfig *terapi.TerraformConfiguration, tfplan *terapi.TerraformPlan) time.Duration {
	if tfconfig.Spec.RepeatEvery == nil {
		return 0
	}

	duration := tfconfig.Spec.RepeatEvery.Duration
	now := time.Now()
	nextMetaTime := metav1.NewTime(now.Add(duration)).Rfc3339Copy()

	if tfplan.Status.LastRunAt != nil {
		last := tfplan.Status.LastRunAt.Rfc3339Copy()
		if timeDiff := now.Sub(last.Time); timeDiff < duration {
			duration -= timeDiff
			nextMetaTime = metav1.NewTime(now.Add(duration)).Rfc3339Copy()
		}
	}

	tfplan.Spec.NextRunAt = &nextMetaTime
	return duration
}

func generateTerraformPlan(tfconf *terapi.TerraformConfiguration, tfplan *terapi.TerraformPlan, scheme *runtime.Scheme) error {
	tfplan.Spec.Approved = tfconf.Spec.AutoApprove
	planScheduleNextRunAt(tfconf, tfplan)
	return ctrl.SetControllerReference(tfconf, tfplan, scheme)
}

// handleFinalizers handles setup / removal of finalizer
// `ok == false` signalize to calling function to return
func (r *TerraformConfigurationReconciler) handleFinalizers(ctx context.Context,
	tfconf *terapi.TerraformConfiguration,
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
