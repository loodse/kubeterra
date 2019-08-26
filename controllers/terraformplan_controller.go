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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	khash "k8s.io/kubernetes/pkg/util/hash"
	"k8s.io/utils/pointer"
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
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformlogs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.kubeterra.io,resources=terraformlogs/status,verbs=get;update;patch
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
		return ctrl.Result{}, err
	}

	if !tfplan.GetDeletionTimestamp().IsZero() {
		// TODO cleanup stuff
		return ctrl.Result{}, nil
	}

	if tfplan.Status.Phase == "" {
		// incomplete plan
		// wait for status update from TerraformConfigurationReconciler
		return ctrl.Result{}, nil
	}

	if tfplan.Spec.Template == nil {
		tfplan.Spec.Template = &terraformv1alpha1.TerraformConfigurationTemplate{}
	}

	currentSpecHash := deepHashObject(tfplan.Spec)
	planSpecChanged := currentSpecHash != tfplan.Status.SpecHash
	tfplan.Status.SpecHash = currentSpecHash
	if err := r.Status().Update(ctx, &tfplan); err != nil {
		log.Info("can't update tfplan")
		return ctrl.Result{}, err
	}

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
	}

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
			podsToDelete = append(podsToDelete, pod)
		case r.terraformRunFinished(pod):
			podsToDelete = append(podsToDelete, pod)
			tflogKey := client.ObjectKey{Name: hashedName(&tfplan), Namespace: tfplan.Namespace}
			err := r.Get(ctx, tflogKey, &terraformv1alpha1.TerraformLog{})
			switch {
			case apierrors.IsNotFound(err):
				tflog, err2 := r.fetchLogsAndGenerateTerraformLog(&tfplan, pod)
				if err2 != nil {
					return ctrl.Result{}, err2
				}
				if err3 := r.Create(ctx, tflog); err3 != nil {
					return ctrl.Result{}, err3
				}
			case err == nil:
				// tflog already exists, do nothing
			default:
				return ctrl.Result{}, err
			}
		case !podInPhase(pod, corev1.PodPending, corev1.PodRunning):
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

	if err := mgrIndexer.IndexField(&terraformv1alpha1.TerraformLog{}, indexOwnerKey, indexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.TerraformPlan{}).
		Owns(&corev1.Pod{}).
		Complete(r)
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

func generatePod(tfplan *terraformv1alpha1.TerraformPlan) *corev1.Pod {
	scriptToRun := resources.TerraformPlanScript
	if tfplan.Spec.Approved {
		scriptToRun = resources.TerraformApplyAutoApproveScript
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: hashedName(tfplan) + "-",
			Namespace:    tfplan.Namespace,
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
					EnvFrom:    tfplan.Spec.Template.EnvFrom,
					Env: append(
						tfplan.Spec.Template.Env,
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
						tfplan.Spec.Template.VolumeMounts,
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
				tfplan.Spec.Template.Volumes,
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

func generateConfigMap(tfplan *terraformv1alpha1.TerraformPlan) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hashedName(tfplan),
			Namespace: tfplan.Namespace,
		},
		Data: map[string]string{
			"main.tf":          tfplan.Spec.Configuration,
			"httpbackend.tf":   resources.TerraformHTTPBackendConfig,
			"terraform.tfvars": tfplan.Spec.Values,
		},
	}
}

func (r *TerraformPlanReconciler) fetchLogsAndGenerateTerraformLog(tfplan *terraformv1alpha1.TerraformPlan, pod corev1.Pod) (*terraformv1alpha1.TerraformLog, error) {
	sinceForever := metav1.Unix(1, 0)
	logsReq := r.PodClient.Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: "terraform",
		SinceTime: &sinceForever,
	})

	terraformLogs, err := logsReq.Stream()
	if err != nil {
		return nil, err
	}
	defer terraformLogs.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, terraformLogs)
	if err != nil {
		return nil, err
	}

	tflog := &terraformv1alpha1.TerraformLog{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hashedName(tfplan),
			Namespace: tfplan.Namespace,
		},
		Spec: terraformv1alpha1.TerraformLogSpec{
			Log: buf.String(),
		},
	}

	return tflog, ctrl.SetControllerReference(tfplan, tflog, r.Scheme)
}

func hashedName(tfplan *terraformv1alpha1.TerraformPlan) string {
	return fmt.Sprintf("%s-%s", tfplan.Name, tfplan.Status.SpecHash)
}

func deepHashObject(obj interface{}) string {
	hasher := fnv.New32a()
	khash.DeepHashObject(hasher, obj)
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
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
