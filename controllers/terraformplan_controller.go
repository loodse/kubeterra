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
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	khash "k8s.io/kubernetes/pkg/util/hash"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/loodse/kubeterra/api/v1alpha1"
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

	var planObj terraformv1alpha1.TerraformPlan
	if err := r.Get(ctx, req.NamespacedName, &planObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errLogMsg(err, "unable to fetch TerraformPlan")
	}

	var podList corev1.PodList
	err := r.List(ctx, &podList, client.InNamespace(req.Namespace), client.MatchingFields{indexOwnerKey: req.Name})
	if err != nil {
		return ctrl.Result{}, errLogMsg(err, "unable to list owned Pods")
	}

	currentSpecHash := hashTerraformPlanSpec(planObj.Spec)
	noNeedNewPod := currentSpecHash == planObj.Status.SpecHash
	planObj.Status.SpecHash = currentSpecHash

	if !noNeedNewPod {
		pod := generatePod(req.NamespacedName, currentSpecHash)

		if err := ctrl.SetControllerReference(&planObj, pod, r.Scheme); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to set controller reference")
		}

		if err := r.Create(ctx, pod); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to create pod")
		}

		if err := r.Status().Update(ctx, &planObj); err != nil {
			return ctrl.Result{}, errLogMsg(err, "unable to update TerraformPlan status")
		}
	}

	prefix := fmt.Sprintf("%s-%s-", planObj.Name, currentSpecHash)
	for _, p := range podList.Items {
		if !strings.HasPrefix(p.Name, prefix) {
			p := p
			log.Info("pod name has no current preffix, delete", "pod", p.Name)
			if err := r.Delete(ctx, &p); err != nil {
				return ctrl.Result{}, errLogMsg(err, "unable to delete pod")
			}
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
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func generatePod(name types.NamespacedName, hashName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-%s-", name.Name, hashName),
			Namespace:    name.Namespace,
			Labels: map[string]string{
				"terraform-action": "test",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "terraform",
					Image:   "alpine",
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						`
while true; do
	echo date
	sleep 10
done
						`,
					},
					WorkingDir:   "/terraform/config/mount",
					EnvFrom:      []corev1.EnvFromSource{},
					Env:          []corev1.EnvVar{},
					Resources:    corev1.ResourceRequirements{},
					VolumeMounts: []corev1.VolumeMount{},
				},
				// {
				// 	Name:  "httpbackend",
				// 	Image: "kubermatic/kubeterra:dev",
				// 	Command: []string{
				// 		"/manager",
				// 		"backend",
				// 		"--name",
				// 		name.Name,
				// 		"--namespace",
				// 		name.Namespace,
				// 	},
			},
			Volumes:       []corev1.Volume{},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func generateConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name.Name + "-",
			Namespace:    name.Namespace,
		},
		Data:       map[string]string{},
		BinaryData: map[string][]byte{},
	}
}

func hashTerraformPlanSpec(spec terraformv1alpha1.TerraformPlanSpec) string {
	hasher := fnv.New32a()
	khash.DeepHashObject(hasher, spec)
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}
