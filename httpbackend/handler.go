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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/loodse/kubeterra/api/v1alpha1"
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
		h.pullState(w, r)
	case "POST":
		h.pushState(w, r)
	case "LOCK":
		h.lockState(w, r)
	case "UNLOCK":
		h.unlockState(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *backendHandler) pullState(w http.ResponseWriter, _ *http.Request) {
	state, err := h.getState()
	if err != nil {
		apiErr := extractAPIError(err)
		http.Error(w, apiErr.msg, apiErr.code)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s", state.Spec.State.Raw)
}

func (h *backendHandler) pushState(w http.ResponseWriter, r *http.Request) {
	lockID := r.URL.Query().Get("ID")
	if lockID == "" {
		http.Error(w, "empty LOCK ID", http.StatusBadRequest)
		return
	}

	// TODO: replace ReadAll
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	incomingState := stateModel{}
	err = json.Unmarshal(buf, &incomingState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := h.getState()
	if err != nil {
		apiErr := extractAPIError(err)
		http.Error(w, apiErr.msg, apiErr.code)
		return
	}

	existingState := stateModel{}
	if err := json.Unmarshal(buf, &existingState); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if incomingState.Lineage != existingState.Lineage {
		http.Error(w, "alien state lineage", http.StatusInternalServerError)
		return
	}

	if state.Status.LockID != lockID {
		http.Error(w, "Locked", http.StatusLocked)
		return
	}

	state.Spec.State.Raw = buf
	// TODO: try to figure out retryable errors and retry
	if err := h.Update(h.ctx, state); err != nil {
		apiErr := extractAPIError(err)
		http.Error(w, apiErr.msg, apiErr.code)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *backendHandler) lockState(w http.ResponseWriter, r *http.Request) {
	li := lockInfo{}
	err := json.NewDecoder(r.Body).Decode(&li)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer r.Body.Close()

	state, err := h.getState()
	if err != nil {
		apiErr := extractAPIError(err)
		http.Error(w, apiErr.msg, apiErr.code)
		return
	}

	switch {
	case li.ID == "":
		http.Error(w, "Unknown lock ID", http.StatusBadRequest)
	case li.ID != state.Status.LockID:
		http.Error(w, "LOCKED", http.StatusLocked)
	case state.Status.LockID == "":
		now := metav1.Now()
		state.Status.LockID = li.ID
		state.Status.LockedSince = &now
		if err := h.Status().Update(h.ctx, state); err != nil {
			apiErr := extractAPIError(err)
			http.Error(w, apiErr.msg, apiErr.code)
			return
		}

		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Unknown state", http.StatusLocked)
	}
}

func (h *backendHandler) unlockState(w http.ResponseWriter, r *http.Request) {
	li := lockInfo{}
	err := json.NewDecoder(r.Body).Decode(&li)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer r.Body.Close()

	state, err := h.getState()
	if err != nil {
		apiErr := extractAPIError(err)
		http.Error(w, apiErr.msg, apiErr.code)
		return
	}

	switch {
	case li.ID == "":
		http.Error(w, "Unknown lock ID", http.StatusBadRequest)
	case li.ID == state.Status.LockID:
		state.Status.LockID = ""
		state.Status.LockedSince = nil
		if err := h.Status().Update(h.ctx, state); err != nil {
			apiErr := extractAPIError(err)
			http.Error(w, apiErr.msg, apiErr.code)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "LOCKED", http.StatusLocked)
	}
}

func (h *backendHandler) getState() (*terraformv1alpha1.TerraformState, error) {
	state := &terraformv1alpha1.TerraformState{}
	stateKey := client.ObjectKey{Name: h.name, Namespace: ""}

	if err := h.Get(h.ctx, stateKey, state); err != nil {
		return nil, err
	}

	if state.Spec.State == nil {
		return nil, errors.New("state is empty")
	}

	return state, nil
}

func extractAPIError(err error) *httpAPIError {
	apiErr := &httpAPIError{}

	switch errAPI := err.(type) {
	case apierrors.APIStatus:
		apiErr.code = int(errAPI.Status().Code)
		apiErr.msg = fmt.Sprintf("%s: %s", errAPI.Status().Reason, errAPI.Status().Message)
	case nil:
		apiErr.code = http.StatusInternalServerError
		apiErr.msg = "nil error"
	default:
		apiErr.code = http.StatusInternalServerError
		apiErr.msg = err.Error()
	}

	return apiErr
}

type lockInfo struct {
	ID        string
	Operation string
	Info      string
	Who       string
	Version   string
	Created   string
	Path      string
}

type stateModel struct {
	Version int    `json:"version"`
	Lineage string `json:"lineage"`
	Serial  int    `json:"serial"`
}

type httpAPIError struct {
	code int
	msg  string
}
