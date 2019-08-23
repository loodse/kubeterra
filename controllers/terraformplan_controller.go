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
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	khash "k8s.io/kubernetes/pkg/util/hash"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/loodse/kubeterra/api/v1alpha1"
	"github.com/loodse/kubeterra/resources"
)

// TerraformPlanReconciler reconciles a TerraformPlan object
type TerraformPlanReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	PodClient corev1typed.PodsGetter
}

// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=pods/logs,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile state
func (r *TerraformPlanReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("terraformplan", req.NamespacedName)
	errLogMsg := logError(log)

	var tfplan terraformv1alpha1.TerraformPlan
	if err := r.Get(ctx, req.NamespacedName, &tfplan); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errLogMsg(err, "unable to fetch TerraformPlan")
	}

	currentSpecHash := deepHashObject(tfplan.Spec)
	planSpecChanged := currentSpecHash != tfplan.Status.SpecHash
	tfplan.Status.SpecHash = currentSpecHash

	if planSpecChanged {
		pod := generatePod(&tfplan)
		cm := generateConfigMap(&tfplan)

		if err := ctrl.SetControllerReference(&tfplan, pod, r.Scheme); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to set pod controller reference", "pod", pod.Name)
		}

		if err := ctrl.SetControllerReference(&tfplan, cm, r.Scheme); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to set configmap controller reference", "configmap", cm.Name)
		}

		if err := r.Create(ctx, cm); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to create configMap", "configmap", cm.Name)
		}

		if err := r.Create(ctx, pod); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to create pod", "pod", pod.Name)
		}

		if err := r.Status().Update(ctx, &tfplan); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to update TerraformPlan status")
		}
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(req.Namespace), client.MatchingFields{indexOwnerKey: req.Name}); err != nil {
		return ctrl.Result{}, errLogMsg(err, "unable to list owned Pods")
	}

	prefix := fmt.Sprintf("%s-%s", tfplan.Name, currentSpecHash)
	podsToDelete := []corev1.Pod{}
	for _, p := range podList.Items {
		if !strings.HasPrefix(p.Name, prefix) {
			podsToDelete = append(podsToDelete, p)
			log.Info("pod name has no current preffix, delete", "pod", p.Name)
			// needed to avoid placing it second time in toDelete slice
			continue
		}

		for _, contStatus := range p.Status.ContainerStatuses {
			if contStatus.Name == "terraform" && contStatus.State.Terminated != nil {
				// terraform finished
				// grab the logs
				sinceForever := metav1.Unix(1, 0)
				logsReq := r.PodClient.Pods(p.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{
					Container: "terraform",
					SinceTime: &sinceForever,
				})
				terraformLogs, err := logsReq.Stream()
				if err != nil {
					return ctrl.Result{}, errLogMsg(err, "unable to stream logs from pod")
				}
				defer terraformLogs.Close()

				var buf bytes.Buffer
				_, err = io.Copy(&buf, terraformLogs)
				if err != nil {
					return ctrl.Result{}, errLogMsg(err, "unable to copy logs from pod")
				}

				tfplan.Status.Logs = buf.String()
				if err := r.Status().Update(ctx, &tfplan); err != nil {
					return ctrl.Result{}, errLogMsg(err, "unable to update TerraformPlan.Status")
				}

				podsToDelete = append(podsToDelete, p)
				continue
			}
		}

		switch p.Status.Phase {
		case corev1.PodPending, corev1.PodRunning:
		default:
			log.Info("pod in non running phase, delete", "pod", p.Name, "phase", p.Status.Phase)
			podsToDelete = append(podsToDelete, p)
		}
	}

	for _, p := range podsToDelete {
		pod := p
		if cmName, ok := pod.Annotations[resources.LinkedTerraformConfigMapAnnotation]; ok {
			cmKey := metav1.ObjectMeta{
				Name:      cmName,
				Namespace: pod.Namespace,
			}
			if err := r.Delete(ctx, &corev1.ConfigMap{ObjectMeta: cmKey}); err != nil {
				return ctrl.Result{}, errLogMsg(err, "unable to delete configMap", "name", cmName)
			}
		}

		if err := r.Delete(ctx, &pod); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to delete pod")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager dependency inject controller
func (r *TerraformPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mgrIndexer := mgr.GetFieldIndexer()
	indexer := indexerFunc("TerraformPlan", terraformv1alpha1.GroupVersion.String())

	if err := mgrIndexer.IndexField(&corev1.Pod{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	if err := mgrIndexer.IndexField(&corev1.ConfigMap{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.TerraformPlan{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func generatePod(tfPlan *terraformv1alpha1.TerraformPlan) *corev1.Pod {
	hashedName := fmt.Sprintf("%s-%s", tfPlan.Name, tfPlan.Status.SpecHash)
	scriptToRun := resources.TerraformPlanScript
	if tfPlan.Spec.Approved {
		scriptToRun = resources.TerraformApplyAutoApproveScript
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: hashedName + "-",
			Namespace:    tfPlan.Namespace,
			Annotations: map[string]string{
				resources.LinkedTerraformConfigMapAnnotation: hashedName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "terraform",
					Image:   resources.Image,
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						scriptToRun,
					},
					WorkingDir: "/terraform/config/mount",
					EnvFrom:    tfPlan.Spec.Template.EnvFrom,
					Env:        tfPlan.Spec.Template.Env,
					Resources:  corev1.ResourceRequirements{},
					VolumeMounts: append(
						tfPlan.Spec.Template.VolumeMounts,
						corev1.VolumeMount{
							Name:      "tfconfig",
							MountPath: "/terraform/config/mount",
						},
					),
				},
				{
					Name:  "httpbackend",
					Image: resources.Image,
					Command: []string{
						"/manager",
						"backend",
						"--name",
						tfPlan.Name,
						"--namespace",
						tfPlan.Namespace,
					},
				},
			},
			Volumes: append(
				tfPlan.Spec.Template.Volumes,
				corev1.Volume{
					Name: "tfconfig",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: hashedName,
							},
						},
					},
				},
			),
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func generateConfigMap(tfPlan *terraformv1alpha1.TerraformPlan) *corev1.ConfigMap {
	hashedName := fmt.Sprintf("%s-%s", tfPlan.Name, tfPlan.Status.SpecHash)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hashedName,
			Namespace: tfPlan.Namespace,
		},
		Data: map[string]string{
			"main.tf":          tfPlan.Spec.Configuration,
			"httpbackend.tf":   resources.TerraformHTTPBackendConfig,
			"terraform.tfvars": tfPlan.Spec.Values,
		},
	}
}

func deepHashObject(obj interface{}) string {
	hasher := fnv.New32a()
	khash.DeepHashObject(hasher, obj)
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}
