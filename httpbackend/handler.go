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
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type backendHandler struct {
	client.Client
	Log logr.Logger
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
	w.WriteHeader(http.StatusOK)
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
