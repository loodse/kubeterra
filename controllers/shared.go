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

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	khash "k8s.io/kubernetes/pkg/util/hash"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	indexOwnerKey = ".metadata.controller"
)

// logError returns closure which checks error != nil and call log.Error on it.
// error object will be returned without changes
func logError(log logr.Logger) func(error, string, ...interface{}) error {
	return func(err error, msg string, keyValues ...interface{}) error {
		if err != nil {
			log.Info(msg, append([]interface{}{"error", err.Error()}, keyValues...)...)
		}
		return err
	}
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

func indexerFunc(ownerKind, ownerAPIGV string) func(runtime.Object) []string {
	return func(obj runtime.Object) []string {
		metaObj, ok := obj.(metav1.Object)
		if !ok {
			return nil
		}

		owner := metav1.GetControllerOf(metaObj)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != ownerAPIGV || owner.Kind != ownerKind {
			return nil
		}

		return []string{owner.Name}
	}
}

func deepHashObject(obj interface{}) string {
	hasher := fnv.New32a()
	khash.DeepHashObject(hasher, obj)
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

var noopGenerator = func() error { return nil }

// findOrCreate tries to `client.Get` object, in case if it's absent —
// initialize new object with `generator` and `client.Create` it
func findOrCreate(ctx context.Context, cli client.Client, obj runtime.Object, generator func() error) (bool, error) {
	var created bool

	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return created, err
	}

	err = cli.Get(ctx, key, obj)
	if apierrors.IsNotFound(err) {
		if err = generator(); err != nil {
			return created, err
		}
		created = true
		return created, cli.Create(ctx, obj)
	}
	return created, err
}
