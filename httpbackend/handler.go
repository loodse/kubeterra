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

package httpbackend

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/kubermatic/kubeterra/api/v1alpha1"
)

type backendHandler struct {
	client.Client
	log  logr.Logger
	name string
	ctx  context.Context
}

func (h *backendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		h.getHandler(w, r)
	case "POST":
		h.postHandler(w, r)
	case "LOCK":
		h.lockHandler(w, r)
	case "UNLOCK":
		h.unlockHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *backendHandler) getHandler(w http.ResponseWriter, r *http.Request) {
	state, err := h.getState()
	if err != nil {
		http.Error(w, err.msg, err.code)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s", state.Spec.State.Raw)
}

func (h *backendHandler) postHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *backendHandler) lockHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *backendHandler) unlockHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *backendHandler) getState() (*terraformv1alpha1.TerraformState, *httpError) {
	state := &terraformv1alpha1.TerraformState{}
	stateKey := client.ObjectKey{Name: h.name, Namespace: ""}
	err := h.Get(h.ctx, stateKey, state)

	if err != nil {
		errHttp := &httpError{}
		if apierrors.IsNotFound(err) {
			errHttp.code = http.StatusNotFound
			errHttp.msg = "404 TerraformState not found"
		} else {
			errHttp.code = http.StatusInternalServerError
			errHttp.msg = err.Error()
		}
		return nil, errHttp
	}

	return state, nil
}

type tfLockInfoModel struct {
	ID        string
	Operation string
	Info      string
	Who       string
	Version   string
	Created   string
	Path      string
}

type tfStateModel struct {
	Version int    `json:"version"`
	Lineage string `json:"lineage"`
	Serial  int    `json:"serial"`
}

type httpError struct {
	code int
	msg  string
}
