/*
Copyright 2019 The KubeTerra Authors.

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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terapi "github.com/loodse/kubeterra/api/v1alpha1"
	"github.com/loodse/kubeterra/resources"
)

// TerraformPlanReconciler reconciles a TerraformPlan object
type TerraformPlanReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	PodClient corev1typed.PodsGetter
}

// SetupWithManager dependency inject controller
func (r *TerraformPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mgrIndexer := mgr.GetFieldIndexer()
	indexer := indexerFunc("TerraformPlan", terapi.GroupVersion.String())

	if err := mgrIndexer.IndexField(&corev1.Pod{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	if err := mgrIndexer.IndexField(&corev1.ConfigMap{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terapi.TerraformPlan{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans,verbs=*
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans/status,verbs=*
// +kubebuilder:rbac:groups=core,resources=pods,verbs=*
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=*
// +kubebuilder:rbac:groups=core,resources=pods/log,verbs=*
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=*

// Reconcile state
func (r *TerraformPlanReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) { //nolint:gocyclo
	ctx := context.Background()
	log := r.Log.WithValues("terraformplan", req.NamespacedName)
	errLogMsg := logError(log)
	defer log.Info("done")

	log.Info("get TerraformPlan")
	var tfplan terapi.TerraformPlan
	if err := r.Get(ctx, req.NamespacedName, &tfplan); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("TerraformPlan not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if tfplan.Status.Phase == "" {
		log.Info("phase is empty")
		tfplan.Status.Phase = terapi.TerraformPhasePlanScheduled
		return ctrl.Result{}, errLogMsg(r.Status().Update(ctx, &tfplan), "failed to update TerraformPlan.Status")
	}

	if !tfplan.GetDeletionTimestamp().IsZero() {
		log.Info("TerraformPlan is being deleted")
		// TODO cleanup stuff
		return ctrl.Result{}, nil
	}

	log.Info("get TerraformConfiguration")
	var tfconfig terapi.TerraformConfiguration
	if err := r.Get(ctx, req.NamespacedName, &tfconfig); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("TerraformConfiguration not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if tfconfig.Spec.Paused {
		log.Info("TerraformConfiguration is paused")
		return ctrl.Result{}, nil
	}

	now := metav1.Now().Rfc3339Copy()
	currentSpecHash := deepHashObject(tfconfig.Spec)
	tfconfSpecChanged := tfplan.Status.ConfigurationSpecHash != currentSpecHash
	scheduleTrigger := false
	tfplan.Status.ConfigurationSpecHash = currentSpecHash

	if tfconfig.Spec.RepeatEvery != nil {
		if tfplan.Spec.NextRunAt != nil {
			next := tfplan.Spec.NextRunAt.Rfc3339Copy()

			switch {
			case next.Before(&now):
				scheduleTrigger = true
			case tfplan.Status.LastRunAt == nil:
				scheduleTrigger = true
			case tfplan.Status.LastRunAt != nil:
				scheduleTrigger = now.Sub(tfplan.Status.LastRunAt.Time) >= tfconfig.Spec.RepeatEvery.Duration
			}
		}
	}

	newRunRequested := tfconfSpecChanged || scheduleTrigger

	if tfconfig.Spec.Template == nil {
		// work around NPE
		tfconfig.Spec.Template = &terapi.TerraformConfigurationTemplate{}
	}

	log.Info("params", "tfconfSpecChanged", tfconfSpecChanged, "runScheduled", scheduleTrigger)

	if newRunRequested {
		if tfconfSpecChanged {
			log.Info("hash TerraformConfiguration.Spec has changed")
		}
		if scheduleTrigger {
			log.Info("TerraformPlan.Spec.NextRunAt triggered")
		}

		log.Info("generate terraform pod")
		pod := generatePod(&tfconfig, &tfplan)

		log.Info("generate terraform configMap")
		cm := generateConfigMap(&tfconfig, &tfplan)

		if err := ctrl.SetControllerReference(&tfplan, pod, r.Scheme); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to set pod controller reference", "pod", pod.Name)
		}

		if err := ctrl.SetControllerReference(&tfplan, cm, r.Scheme); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to set configmap controller reference", "configmap", cm.Name)
		}

		log.Info("create terraform configMap")
		if err := r.Create(ctx, cm); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, errLogMsg(err, "unable to create configMap", "configmap", cm.Name)
			}
		}

		log.Info("create terraform pod")
		if err := r.Create(ctx, pod); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, errLogMsg(err, "unable to create pod", "pod", pod.Name)
			}
		}

		log.Info("update TerraformPlan.Status")

		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if _, err := findOrCreate(ctx, r.Client, &tfplan, noopGenerator); err != nil {
				return err
			}
			lastRunAt := metav1.Now()
			tfplan.Status.ConfigurationSpecHash = currentSpecHash
			tfplan.Status.LastRunAt = &lastRunAt
			return r.Status().Update(ctx, &tfplan)
		})

		return ctrl.Result{}, errLogMsg(retryErr, "can't update TerraformPlan.Status")
	}

	log.Info("podList")

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(req.Namespace), client.MatchingFields{indexOwnerKey: req.Name}); err != nil {
		return ctrl.Result{}, errLogMsg(err, "unable to list owned Pods")
	}

	prefix := hashedName(&tfplan)
	podsToDelete := []corev1.Pod{}

	for _, p := range podList.Items {
		pod := p
		switch {
		case !strings.HasPrefix(pod.Name, prefix):
			log.Info("pod has wrong prefix")
			podsToDelete = append(podsToDelete, pod)
		case r.terraformRunFinished(pod):
			log.Info("terraform pod finished")
			podsToDelete = append(podsToDelete, pod)

			// logCM := &corev1.ConfigMap{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      hashedName(&tfplan),
			// 		Namespace: tfplan.Namespace,
			// 	},
			// }

			// created, err := findOrCreate(ctx, r.Client, logCM, func() error {
			// 	logConfigMap, errGenerate := r.generateTerraformLog(&tfplan, pod)
			// 	if errGenerate != nil {
			// 		return errGenerate
			// 	}
			// 	logConfigMap.DeepCopyInto(logCM)
			// 	return nil
			// })
			// if err != nil {
			// 	return ctrl.Result{}, err
			// }
			// if created {
			// 	log.Info("terraform logs saved", "namespace", logCM.Namespace, "configMap", logCM.Name)
			// }
		case !podInPhase(pod, corev1.PodPending, corev1.PodRunning):
			log.Info("pod is not in pending or running phase", "phase", pod.Status.Phase)
			podsToDelete = append(podsToDelete, pod)
		}
	}

	for _, p := range podsToDelete {
		pod := p
		if cmName, ok := pod.Annotations[resources.LinkedTerraformConfigMapAnnotation]; ok {
			cmKey := metav1.ObjectMeta{
				Name:      cmName,
				Namespace: pod.Namespace,
			}
			err := ignoreAPIErrors(r.Delete(ctx, &corev1.ConfigMap{ObjectMeta: cmKey}), apierrors.IsNotFound, apierrors.IsGone)
			if err != nil {
				return ctrl.Result{}, errLogMsg(err, "unable to delete configMap")
			}
		}

		err := ignoreAPIErrors(r.Delete(ctx, &pod), apierrors.IsNotFound, apierrors.IsGone)
		if err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to delete pod")
		}
	}

	return ctrl.Result{}, nil
}

func (r *TerraformPlanReconciler) terraformRunFinished(pod corev1.Pod) bool {
	for _, contStatus := range pod.Status.ContainerStatuses {
		if contStatus.Name == "terraform" && contStatus.State.Terminated != nil {
			return true
		}
	}
	return false
}

func podInPhase(pod corev1.Pod, phases ...corev1.PodPhase) bool {
	for _, phase := range phases {
		if pod.Status.Phase == phase {
			return true
		}
	}
	return false
}

func generatePod(tfconfig *terapi.TerraformConfiguration, tfplan *terapi.TerraformPlan) *corev1.Pod {
	scriptToRun := resources.TerraformPlanScript
	if tfplan.Spec.Approved {
		scriptToRun = resources.TerraformApplyAutoApproveScript
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hashedName(tfplan),
			Namespace: tfplan.Namespace,
			Annotations: map[string]string{
				resources.LinkedTerraformConfigMapAnnotation: hashedName(tfplan),
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: pointer.BoolPtr(true),
			},
			Containers: []corev1.Container{
				{
					Name:    "terraform",
					Image:   resources.Image,
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						shellCMD(scriptToRun),
					},
					WorkingDir: "/terraform/config",
					EnvFrom:    tfconfig.Spec.Template.EnvFrom,
					Env: append(
						tfconfig.Spec.Template.Env,
						corev1.EnvVar{
							Name:  "TF_DATA_DIR",
							Value: "/tmp/tfdata",
						},
						corev1.EnvVar{
							Name:  "TF_IN_AUTOMATION",
							Value: "1",
						},
					),
					Resources: corev1.ResourceRequirements{},
					VolumeMounts: append(
						tfconfig.Spec.Template.VolumeMounts,
						corev1.VolumeMount{
							Name:      "tfconfig",
							MountPath: "/terraform/config",
						},
					),
				},
				{
					Name:  "httpbackend",
					Image: resources.Image,
					Command: []string{
						"/kubeterra",
						"backend",
						"--name",
						tfplan.Name,
						"--namespace",
						tfplan.Namespace,
					},
				},
			},
			Volumes: append(
				tfconfig.Spec.Template.Volumes,
				corev1.Volume{
					Name: "tfconfig",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: hashedName(tfplan),
							},
							Optional: pointer.BoolPtr(false),
						},
					},
				},
			),
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func generateConfigMap(tfconfig *terapi.TerraformConfiguration, tfplan *terapi.TerraformPlan) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hashedName(tfplan),
			Namespace: tfplan.Namespace,
		},
		Data: map[string]string{
			"main.tf":          tfconfig.Spec.Configuration,
			"terraform.tfvars": tfconfig.Spec.Values,
		},
	}
}

// func (r *TerraformPlanReconciler) generateTerraformLog(tfplan *terapi.TerraformPlan, pod corev1.Pod) (*corev1.ConfigMap, error) {
// 	sinceForever := metav1.Unix(1, 0)
// 	logsReq := r.PodClient.Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
// 		Container: "terraform",
// 		SinceTime: &sinceForever,
// 	})

// 	terraformLogs, err := logsReq.Stream()
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer terraformLogs.Close()

// 	var buf bytes.Buffer
// 	_, err = io.Copy(&buf, terraformLogs)
// 	if err != nil {
// 		return nil, err
// 	}

// 	tflog := &corev1.ConfigMap{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      hashedName(tfplan),
// 			Namespace: tfplan.Namespace,
// 		},
// 		Data: map[string]string{
// 			"logs": buf.String(),
// 		},
// 	}

// 	return tflog, ctrl.SetControllerReference(tfplan, tflog, r.Scheme)
// }

func hashedName(tfplan *terapi.TerraformPlan) string {
	return fmt.Sprintf("%s-%s", tfplan.Name, tfplan.Status.ConfigurationSpecHash)
}

func shellCMD(cmdLines ...string) string {
	return strings.Join(append([]string{`set -exuf -o pipefail`}, cmdLines...), "\n")
}

func ignoreAPIErrors(err error, checks ...func(error) bool) error {
	for _, check := range checks {
		if check(err) {
			return nil
		}
	}
	return err
}
