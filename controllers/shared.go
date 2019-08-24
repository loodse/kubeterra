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
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
