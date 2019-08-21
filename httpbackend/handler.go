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
	log       logr.Logger
	ctx       context.Context
	name      string
	namespace string
}

func (h *backendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var err error
	switch r.Method {
	case "GET":
		err = h.pullState(w, r)
	case "POST":
		err = h.pushState(w, r)
	case "LOCK":
		err = h.lockState(w, r)
	case "UNLOCK":
		err = h.unlockState(w, r)
	default:
		err = &httpAPIError{code: http.StatusNotFound, msg: "404 page not found"}
	}

	if err != nil {
		apiErr := extractAPIError(err)
		http.Error(w, apiErr.msg, apiErr.code)
	}
}

func (h *backendHandler) pullState(w http.ResponseWriter, _ *http.Request) error {
	state, err := h.getState()
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "%s", state.Spec.State.Raw)
	return err
}

func (h *backendHandler) pushState(w http.ResponseWriter, r *http.Request) error {
	lockID := r.URL.Query().Get("ID")
	if lockID == "" {
		return &httpAPIError{code: http.StatusBadRequest, msg: "empty LOCK ID"}
	}

	// TODO: replace ReadAll
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	incomingState := stateModel{}
	err = json.Unmarshal(buf, &incomingState)
	if err != nil {
		return err
	}

	state, err := h.getState()
	if err != nil {
		return err
	}

	existingState := stateModel{}
	if err := json.Unmarshal(buf, &existingState); err != nil {
		return err
	}

	if incomingState.Lineage != existingState.Lineage {
		return &httpAPIError{code: http.StatusBadRequest, msg: "alien state lineage"}
	}

	if state.Status.LockID != lockID {
		return &httpAPIError{code: http.StatusLocked, msg: "locked"}
	}

	state.Spec.State.Raw = buf
	// TODO: try to figure out retryable errors and retry
	if err := h.Update(h.ctx, state); err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func (h *backendHandler) lockState(w http.ResponseWriter, r *http.Request) error {
	li := lockInfo{}
	err := json.NewDecoder(r.Body).Decode(&li)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	state, err := h.getState()
	if err != nil {
		return err
	}

	switch {
	case li.ID == "":
		return &httpAPIError{code: http.StatusBadRequest, msg: "unknown lock ID"}
	case state.Status.LockID != "":
		return &httpAPIError{code: http.StatusLocked, msg: "locked"}
	}

	now := metav1.Now()
	state.Status.LockID = li.ID
	state.Status.LockedSince = &now
	if err := h.Status().Update(h.ctx, state); err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func (h *backendHandler) unlockState(w http.ResponseWriter, r *http.Request) error {
	li := lockInfo{}
	err := json.NewDecoder(r.Body).Decode(&li)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	state, err := h.getState()
	if err != nil {
		return err
	}

	switch {
	case li.ID == "":
		return &httpAPIError{code: http.StatusBadRequest, msg: "unknown lock ID"}
	case li.ID != state.Status.LockID:
		return &httpAPIError{code: http.StatusLocked, msg: "locked"}
	}

	state.Status.LockID = ""
	state.Status.LockedSince = nil
	// TODO: try to figure out retryable errors and retry
	if err := h.Status().Update(h.ctx, state); err != nil {
		return err
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

func (h *backendHandler) getState() (*terraformv1alpha1.TerraformState, error) {
	state := &terraformv1alpha1.TerraformState{}
	stateKey := client.ObjectKey{Name: h.name, Namespace: h.namespace}

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
	case *httpAPIError:
		apiErr.code = errAPI.code
		apiErr.msg = errAPI.msg
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

func (e *httpAPIError) Error() string {
	return fmt.Sprintf("%d %s", e.code, e.msg)
}
