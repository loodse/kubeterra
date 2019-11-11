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

package command

import (
	"github.com/spf13/cobra"

	"github.com/loodse/kubeterra/manager"
)

type managerOptions struct {
	*globalOptions
	Namespace            string
	MetricsAddr          string
	EnableLeaderElection bool
}

func managerCmd(gopts *globalOptions) *cobra.Command {
	opts := managerOptions{
		globalOptions: gopts,
	}

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "controller manager",
		Args:  cobra.NoArgs,
		Long: `
Launch kubernetes controller manager that will watch and act over CRDs:
* TerraformConfiguration
* TerraformPlan
* TerraformState
		`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return manager.Launch(manager.Options{
				MetricsAddr:    opts.MetricsAddr,
				LeaderElection: opts.EnableLeaderElection,
				Development:    opts.Debug,
				Namespace:      opts.Namespace,
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.MetricsAddr, "metrics-addr", ":8080", "the address the metric endpoint binds to.")
	flags.BoolVarP(&opts.EnableLeaderElection, "enable-leader-election", "l", false, "enable leader election for controller manager.")
	flags.StringVar(&opts.Namespace, "namespace", "kubeterra-system", "namespace to watch over")

	return cmd
}
