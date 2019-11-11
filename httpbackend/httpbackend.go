/*
Copyright 2019 The KubeTerra Authors.

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
	"net/http"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	terraformv1alpha1 "github.com/loodse/kubeterra/api/v1alpha1"
)

// Options to configure terraform http backend
type Options struct {
	TerraformStateName      string
	TerraformStateNamespace string
	Listen                  string
	Development             bool
}

// ListenAndServe launch terraform http backend server
func ListenAndServe(opts Options) error {
	ctrl.SetLogger(zap.Logger(opts.Development))
	httpLog := ctrl.Log.WithName("http")
	httpLog.Info("starting", "port", opts.Listen, "state-name", opts.TerraformStateName)

	mux, err := newHTTPBackendMux(opts.TerraformStateName, opts.TerraformStateNamespace, httpLog)
	if err != nil {
		return err
	}

	return http.ListenAndServe(opts.Listen, mux)
}

func newHTTPBackendMux(name, namespace string, httpLog logr.Logger) (*http.ServeMux, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = terraformv1alpha1.AddToScheme(scheme)
	cfg := ctrl.GetConfigOrDie()

	dynClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	h := &backendHandler{
		Client:    dynClient,
		log:       httpLog,
		name:      name,
		namespace: namespace,
		ctx:       context.Background(),
	}

	mux := http.NewServeMux()
	mux.Handle("/", h)
	return mux, nil
}
