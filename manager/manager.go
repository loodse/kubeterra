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

package manager

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// +kubebuilder:scaffold:imports

	terraformv1alpha1 "github.com/loodse/kubeterra/api/v1alpha1"
	"github.com/loodse/kubeterra/controllers"
)

func Launch(metricsAddr string, leaderElection bool) error {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = terraformv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme

	setupLog := ctrl.Log.WithName("setup")
	ctrl.SetLogger(zap.Logger(true))

	syncPeriod := 10 * time.Minute
	mgr, err := ctrl.NewManager(
		ctrl.GetConfigOrDie(),
		ctrl.Options{
			SyncPeriod:         &syncPeriod,
			Scheme:             scheme,
			MetricsBindAddress: metricsAddr,
			LeaderElection:     leaderElection,
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	restConfig := mgr.GetConfig()
	coreV1Client, err := corev1typed.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to init corev1 client", "client", "corev1typed")
		os.Exit(1)
	}

	if err = (&controllers.TerraformPlanReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("TerraformPlan"),
		Scheme:    mgr.GetScheme(),
		PodClient: coreV1Client,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TerraformPlan")
		os.Exit(1)
	}

	if err = (&controllers.TerraformConfigurationReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("TerraformConfiguration"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TerraformConfiguration")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil
}
