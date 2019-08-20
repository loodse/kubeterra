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

package command

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kubermatic/kubeterra/manager"
)

type managerOpts struct {
	GlobalOpts           `mapstructure:",squash"`
	MetricsAddr          string `mapstructure:"metrics-addr"`
	EnableLeaderElection bool   `mapstructure:"enable-leader-election"`
}

func managerCmd() *cobra.Command {
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
		RunE: runManager,
	}

	flags := cmd.Flags()

	// flags declared here should be cosistent with managerOpts structure
	flags.String("metrics-addr", ":8080", "the address the metric endpoint binds to.")
	flags.BoolP("enable-leader-election", "l", false, "enable leader election for controller manager.")

	_ = viper.BindPFlags(flags)
	return cmd
}

func runManager(cmd *cobra.Command, _ []string) error {
	var opts managerOpts

	if err := viper.Unmarshal(&opts); err != nil {
		return err
	}

	return manager.Launch(opts.MetricsAddr, opts.EnableLeaderElection)
}
