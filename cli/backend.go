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

	"github.com/loodse/kubeterra/httpbackend"
)

type backendOptions struct {
	*globalOptions
	Name      string
	Namespace string
	Listen    string
}

func backendCmd(gopts *globalOptions) *cobra.Command {
	opts := backendOptions{
		globalOptions: gopts,
	}

	cmd := &cobra.Command{
		Use:   "backend",
		Args:  cobra.NoArgs,
		Short: "launch terraform HTTP backend",
		Long: `
This process is used as side-car to running terraform http backend. It will
proxy terraform state to TerraformState object.
		`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return httpbackend.ListenAndServe(httpbackend.Options{
				TerraformStateName:      opts.Name,
				TerraformStateNamespace: opts.Namespace,
				Listen:                  opts.Listen,
				Development:             opts.Debug,
			})
		},
	}

	flags := cmd.Flags()

	// flags declared here should be cosistent with backendOpts structure
	flags.StringVarP(&opts.Name, "name", "n", "", "name of the terraform state object to use")
	flags.StringVarP(&opts.Namespace, "namespace", "s", "", "name of the namespace where terraform state object is located")
	flags.StringVarP(&opts.Listen, "listen", "l", "localhost:8081", "listen port")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("namespace")

	return cmd
}
