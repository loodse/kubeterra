/*
Copyright 2019 The kubeterra Authors.

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

package terraformmodule

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	corev1beta1 "github.com/kubermatic/kubeterra/pkg/apis/core/v1beta1"
	"github.com/pkg/errors"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log               = logf.Log.WithName("controller")
	asciiCleanupRegex = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")
)

const (
	destroyFinalizer = "kubeterra.io/destroy"
	updateRetries    = 1
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new TerraformModule Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileTerraformModule{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("terraformmodule-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to TerraformModule
	err = c.Watch(&source.Kind{Type: &corev1beta1.TerraformModule{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch TerraformResource and map them on Modules selecting them
	if err = c.Watch(&source.Kind{Type: &corev1beta1.TerraformResource{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(o handler.MapObject) (requests []reconcile.Request) {
			instance := o.Object.(*corev1beta1.TerraformResource)
			modules, err := getManagingModulesForResource(mgr.GetClient(), instance)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Unable to get TerraformResource %s/%s's Module memberships: %v", instance.Namespace, instance.Name, err))
				return nil
			}

			for _, module := range modules {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: module.Namespace, Name: module.Name}})
			}
			return
		}),
	}); err != nil {
		return err
	}

	// Watch TerraformProvider and map them on Modules selecting them
	if err = c.Watch(&source.Kind{Type: &corev1beta1.TerraformProvider{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(o handler.MapObject) (requests []reconcile.Request) {
			instance := o.Object.(*corev1beta1.TerraformProvider)
			modules, err := getManagingModulesForProvider(mgr.GetClient(), instance)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Unable to get TerraformProvider %s/%s's Module memberships: %v", instance.Namespace, instance.Name, err))
				return nil
			}

			for _, module := range modules {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: module.Namespace, Name: module.Name}})
			}
			return
		}),
	}); err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileTerraformModule{}

// ReconcileTerraformModule reconciles a TerraformModule object
type ReconcileTerraformModule struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a TerraformModule object and makes changes based on the state read
// and what is in the TerraformModule.Spec
func (r *ReconcileTerraformModule) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	ctx := context.TODO()
	module := &corev1beta1.TerraformModule{}
	if err = r.Get(ctx, request.NamespacedName, module); err != nil {
		if kubeerrors.IsNotFound(err) {
			err = nil
			return
		}
		// Error reading the object - requeue the request.
		return
	}

	providerSelector, err := metav1.LabelSelectorAsSelector(module.Spec.ProviderSelector)
	if err != nil {
		return
	}
	providerList := &corev1beta1.TerraformProviderList{}
	if err = r.List(ctx, &client.ListOptions{
		LabelSelector: providerSelector,
	}, providerList); err != nil {
		return
	}

	resourceSelector, err := metav1.LabelSelectorAsSelector(module.Spec.ResourceSelector)
	if err != nil {
		return
	}
	resourceList := &corev1beta1.TerraformResourceList{}
	if err = r.List(ctx, &client.ListOptions{
		LabelSelector: resourceSelector,
	}, resourceList); err != nil {
		return
	}

	// Phase1
	// Check finalizers
	err = storeTerraformModuleUpdate(ctx, r, module,
		func(toUpdate *corev1beta1.TerraformModule) (abort bool, err error) {
			if containsString(toUpdate.Finalizers, destroyFinalizer) {
				abort = true
				return
			}

			toUpdate.Finalizers = append(toUpdate.Finalizers, destroyFinalizer)
			return
		})
	if err != nil {
		return
	}
	for _, provider := range providerList.Items {
		if containsString(provider.Finalizers, destroyFinalizer) {
			continue
		}

		err = storeTerraformProviderUpdate(ctx, r, &provider,
			func(toUpdate *corev1beta1.TerraformProvider) (abort bool, err error) {
				if containsString(toUpdate.Finalizers, destroyFinalizer) {
					abort = true
					return
				}

				toUpdate.Finalizers = append(toUpdate.Finalizers, destroyFinalizer)
				return
			})
		if err != nil {
			return
		}
	}
	for _, resource := range resourceList.Items {
		if containsString(resource.Finalizers, destroyFinalizer) {
			continue
		}

		err = storeTerraformResourceUpdate(ctx, r, &resource,
			func(toUpdate *corev1beta1.TerraformResource) (abort bool, err error) {
				if containsString(toUpdate.Finalizers, destroyFinalizer) {
					abort = true
					return
				}

				toUpdate.Finalizers = append(toUpdate.Finalizers, destroyFinalizer)
				return
			})
		if err != nil {
			return
		}
	}

	// Phase2
	// Build Terraform module
	var deletedProviders []corev1beta1.TerraformProvider
	providerConfig := map[string]interface{}{}
	for _, provider := range providerList.Items {
		if provider.DeletionTimestamp != nil {
			deletedProviders = append(deletedProviders, provider)
			continue
		}

		var parameters map[string]interface{}
		if parameters, err = UnmarshalRuntimeRawExtension(provider.Spec.Parameters); err != nil {
			return
		}
		providerConfig[provider.Spec.Provider] = parameters
	}

	var deletedResources []corev1beta1.TerraformResource
	resourceConfig := map[string]map[string]interface{}{}
	for _, resource := range resourceList.Items {
		if resource.DeletionTimestamp != nil {
			deletedResources = append(deletedResources, resource)
			continue
		}

		var parameters map[string]interface{}
		if parameters, err = UnmarshalRuntimeRawExtension(resource.Spec.Parameters); err != nil {
			return
		}

		if _, ok := resourceConfig[resource.Spec.Type]; !ok {
			resourceConfig[resource.Spec.Type] = map[string]interface{}{}
		}

		resourceConfig[resource.Spec.Type][resource.Name] = parameters
	}

	config := &terraformConfig{
		Provider: providerConfig,
		Resource: resourceConfig,
	}
	mainTFBytes, err := json.Marshal(config)
	if err != nil {
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mainTFBytes))

	// Phase3
	// Delete if needed
	if module.DeletionTimestamp != nil {
		// Burn down everything and remove finalizers
		if module.Status.TerraformState != nil {
			if err = r.runTerraformDestroy(ctx, module, mainTFBytes); err != nil {
				return
			}
		}

		for _, provider := range providerList.Items {
			if containsString(provider.Finalizers, destroyFinalizer) {
				continue
			}

			err = storeTerraformProviderUpdate(ctx, r, &provider,
				func(toUpdate *corev1beta1.TerraformProvider) (abort bool, err error) {
					toUpdate.Finalizers = removeString(toUpdate.Finalizers, destroyFinalizer)
					return
				})
			if err != nil {
				return
			}
		}
		for _, resource := range resourceList.Items {
			err = storeTerraformResourceUpdate(ctx, r, &resource,
				func(toUpdate *corev1beta1.TerraformResource) (abort bool, err error) {
					toUpdate.Finalizers = removeString(toUpdate.Finalizers, destroyFinalizer)
					return
				})
			if err != nil {
				return
			}
		}

		err = storeTerraformModuleUpdate(ctx, r, module,
			func(toUpdate *corev1beta1.TerraformModule) (abort bool, err error) {
				toUpdate.Finalizers = removeString(toUpdate.Finalizers, destroyFinalizer)
				return
			})
		if err != nil {
			return
		}
		return
	}

	// Phase4
	// Run terraform apply
	defer func() {
		if err != nil {
			errString := err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				errString = asciiCleanupRegex.ReplaceAllString(string(exitErr.Stderr), "")
				errString = strings.Trim(errString, " \n")
			}

			_ = storeTerraformModuleUpdate(ctx, r, module,
				func(toUpdate *corev1beta1.TerraformModule) (bool, error) {
					toUpdate.Status.Conditions = []corev1beta1.TerraformModuleCondition{{
						Type:               corev1beta1.TerraformModuleConditionReady,
						Status:             corev1beta1.ConditionFalse,
						Reason:             "Error",
						Message:            errString,
						LastTransitionTime: metav1.Now(),
					}}
					return false, nil
				})
		}
	}()

	if hash == module.Status.CurrentHash {
		log.Info("nothing todo, still up to date",
			"name", module.Name, "namespace", module.Namespace)
	} else {
		// run terraform
		if err = r.runTerraformApply(ctx, module, mainTFBytes); err != nil {
			return
		}
		err = storeTerraformModuleUpdate(ctx, r, module,
			func(toUpdate *corev1beta1.TerraformModule) (abort bool, err error) {
				toUpdate.Status.CurrentHash = hash
				toUpdate.Status.TerraformState = module.Status.TerraformState
				toUpdate.Status.Conditions = []corev1beta1.TerraformModuleCondition{{
					Type:               corev1beta1.TerraformModuleConditionReady,
					Status:             corev1beta1.ConditionTrue,
					Reason:             "TerraformSuccess",
					Message:            "terraform apply success",
					LastTransitionTime: metav1.Now(),
				}}
				return
			})
		if err != nil {
			return
		}
	}

	// Phase5
	// remove leftover finalizers
	for _, provider := range deletedProviders {
		err = storeTerraformProviderUpdate(ctx, r, &provider,
			func(toUpdate *corev1beta1.TerraformProvider) (abort bool, err error) {
				toUpdate.Finalizers = removeString(toUpdate.Finalizers, destroyFinalizer)
				return
			})
		if err != nil {
			return
		}
	}
	for _, resource := range deletedResources {
		err = storeTerraformResourceUpdate(ctx, r, &resource,
			func(toUpdate *corev1beta1.TerraformResource) (abort bool, err error) {
				toUpdate.Finalizers = removeString(toUpdate.Finalizers, destroyFinalizer)
				return
			})
		if err != nil {
			return
		}
	}

	return
}

func (r *ReconcileTerraformModule) setupTerraform(ctx context.Context, module *corev1beta1.TerraformModule, config []byte) (dir string, err error) {
	if dir, err = ioutil.TempDir("", "kubeterra-"); err != nil {
		return
	}

	log.Info("main.json.tf", "content", string(config))
	if err = ioutil.WriteFile(path.Join(dir, "main.json.tf"), config, os.ModePerm); err != nil {
		return
	}

	if module.Status.TerraformState != nil {
		var (
			stateBytes []byte
			state      map[string]interface{}
		)
		state, err = UnmarshalRuntimeRawExtension(module.Status.TerraformState)
		if err != nil {
			err = errors.Wrap(err, "unmarshal state")
			return
		}
		log.Info("state", "state", fmt.Sprintf("%v", state))

		stateBytes, err = json.Marshal(&state)
		if err != nil {
			err = errors.Wrap(err, "marshal state")
			return
		}
		if err = ioutil.WriteFile(path.Join(dir, "terraform.tfstate"), stateBytes, os.ModePerm); err != nil {
			return
		}
	}

	initCmd := exec.Command("terraform", "init")
	initCmd.Dir = dir
	log.Info("running terraform init", "name", module.Name, "namespace", module.Namespace)
	if _, err = initCmd.Output(); err != nil {
		return
	}
	return
}

func (r *ReconcileTerraformModule) runTerraformDestroy(ctx context.Context, module *corev1beta1.TerraformModule, config []byte) (err error) {
	dir, err := r.setupTerraform(ctx, module, config)
	defer os.RemoveAll(dir)
	if err != nil {
		return
	}

	destroyCmd := exec.Command("terraform", "destroy", "-auto-approve")
	destroyCmd.Dir = dir
	log.Info("running terraform destroy", "name", module.Name, "namespace", module.Namespace)
	if _, err = destroyCmd.Output(); err != nil {
		return
	}

	updatedStateBytes, err := ioutil.ReadFile(path.Join(dir, "terraform.tfstate"))
	if err != nil {
		return
	}
	updatedState := map[string]interface{}{}
	if err = json.Unmarshal(updatedStateBytes, &updatedState); err != nil {
		err = errors.Wrap(err, "unmarshal updated state")
		return
	}
	module.Status.TerraformState = &runtime.RawExtension{
		Object: &unstructured.Unstructured{
			Object: updatedState,
		},
	}
	return
}

func (r *ReconcileTerraformModule) runTerraformApply(ctx context.Context, module *corev1beta1.TerraformModule, config []byte) (err error) {
	dir, err := r.setupTerraform(ctx, module, config)
	defer os.RemoveAll(dir)
	if err != nil {
		return
	}

	applyCmd := exec.Command("terraform", "apply", "-auto-approve")
	applyCmd.Dir = dir
	log.Info("running terraform apply", "name", module.Name, "namespace", module.Namespace)
	if _, err = applyCmd.Output(); err != nil {
		return
	}

	updatedStateBytes, err := ioutil.ReadFile(path.Join(dir, "terraform.tfstate"))
	if err != nil {
		return
	}
	updatedState := map[string]interface{}{}
	if err = json.Unmarshal(updatedStateBytes, &updatedState); err != nil {
		err = errors.Wrap(err, "unmarshal updated state")
		return
	}
	module.Status.TerraformState = &runtime.RawExtension{
		Object: &unstructured.Unstructured{
			Object: updatedState,
		},
	}
	return
}

type terraformConfig struct {
	Provider map[string]interface{}            `json:"provider,omitempty"`
	Resource map[string]map[string]interface{} `json:"resource,omitempty"`
}

func getManagingModulesForResource(c client.Client, resource *corev1beta1.TerraformResource) (sets []*corev1beta1.TerraformModule, err error) {
	moduleList := &corev1beta1.TerraformModuleList{}
	if err = c.List(context.TODO(), nil, moduleList); err != nil {
		return
	}
	for _, module := range moduleList.Items {
		var selector labels.Selector
		if selector, err = metav1.LabelSelectorAsSelector(module.Spec.ResourceSelector); err != nil {
			return
		}
		if selector.Matches(labels.Set(resource.Labels)) {
			sets = append(sets, &module)
		}
	}
	return
}

func getManagingModulesForProvider(c client.Client, provider *corev1beta1.TerraformProvider) (sets []*corev1beta1.TerraformModule, err error) {
	moduleList := &corev1beta1.TerraformModuleList{}
	if err = c.List(context.TODO(), nil, moduleList); err != nil {
		return
	}
	for _, module := range moduleList.Items {
		var selector labels.Selector
		if selector, err = metav1.LabelSelectorAsSelector(module.Spec.ResourceSelector); err != nil {
			return
		}
		if selector.Matches(labels.Set(provider.Labels)) {
			sets = append(sets, &module)
		}
	}
	return
}

func storeTerraformModuleUpdate(
	ctx context.Context,
	c client.Client,
	instance *corev1beta1.TerraformModule,
	updateFn func(toUpdate *corev1beta1.TerraformModule) (abort bool, err error),
) error {
	return storeUpdate(
		ctx, c, instance.Name, instance.Namespace, instance,
		func(toUpdate runtime.Object) (abort bool, err error) {
			obj := toUpdate.(*corev1beta1.TerraformModule)
			abort, err = updateFn(obj)
			if !abort && err == nil {
				log.Info("updating TerraformModule", "name", obj.Name, "namespace", obj.Namespace)
			}
			return
		}, updateRetries)
}

func storeTerraformResourceUpdate(
	ctx context.Context,
	c client.Client,
	instance *corev1beta1.TerraformResource,
	updateFn func(toUpdate *corev1beta1.TerraformResource) (abort bool, err error),
) error {
	return storeUpdate(
		ctx, c, instance.Name, instance.Namespace, instance,
		func(toUpdate runtime.Object) (abort bool, err error) {
			obj := toUpdate.(*corev1beta1.TerraformResource)
			abort, err = updateFn(obj)
			if !abort && err == nil {
				log.Info("updating TerraformResource", "name", obj.Name, "namespace", obj.Namespace)
			}
			return
		}, updateRetries)
}

func storeTerraformProviderUpdate(
	ctx context.Context,
	c client.Client,
	instance *corev1beta1.TerraformProvider,
	updateFn func(toUpdate *corev1beta1.TerraformProvider) (abort bool, err error),
) error {
	return storeUpdate(
		ctx, c, instance.Name, instance.Namespace, instance,
		func(toUpdate runtime.Object) (abort bool, err error) {
			obj := toUpdate.(*corev1beta1.TerraformProvider)
			abort, err = updateFn(obj)
			if !abort && err == nil {
				log.Info("updating TerraformProvider", "name", obj.Name, "namespace", obj.Namespace)
			}
			return
		}, updateRetries)
}

func storeUpdate(
	ctx context.Context,
	c client.Client,
	name, namespace string,
	obj runtime.Object,
	updateFn func(toUpdate runtime.Object) (abort bool, err error),
	retries int,
) error {
	toUpdate := obj.DeepCopyObject()

	var updateErr, getErr error
	for i := 0; i < retries; i++ {
		abort, err := updateFn(toUpdate)
		if err != nil {
			return err
		}
		if abort {
			return nil
		}

		if updateErr = c.Update(ctx, toUpdate); updateErr == nil {
			return nil
		}

		// Update the object to get the latest resource version
		if getErr = c.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, toUpdate); getErr != nil {
			return getErr
		}
	}

	return updateErr
}

// generateHash returns a hash of the given object
func generateHash(obj interface{}) (hash string, err error) {
	j, err := json.Marshal(obj)
	if err != nil {
		return
	}
	sum := sha256.Sum256(j)
	hash = fmt.Sprintf("%x", sum[:])
	return
}

// containsString checks if the string slice contains the needle
func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// removeString removes the given string from the slice
func removeString(in []string, toRemove string) (out []string) {
	for _, v := range in {
		if v != toRemove {
			out = append(out, v)
		}
	}
	return
}

// UnmarshalRuntimeRawExtension converts a runtime.RawExtension into map[string]interface{}
func UnmarshalRuntimeRawExtension(in *runtime.RawExtension) (out map[string]interface{}, err error) {
	out = make(map[string]interface{})
	if in != nil && len(in.Raw) > 0 {
		if err = yaml.Unmarshal(in.Raw, &out); err != nil {
			return
		}
	}
	return
}

// MarshalRuntimeRawExtension converts a map[string]interface{} back into a runtime.RawExtention
func MarshalRuntimeRawExtension(in map[string]interface{}) (out *runtime.RawExtension) {
	if in == nil {
		return
	}
	out = &runtime.RawExtension{
		Object: &unstructured.Unstructured{Object: in},
	}
	return
}
