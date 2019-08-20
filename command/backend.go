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

	"github.com/loodse/kubeterra/httpbackend"
)

type backendOpts struct {
	GlobalOpts `mapstructure:",squash"`
	Name       string `mapstructure:"name"`
	Listen     string `mapstructure:"listen"`
}

func backendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backend",
		Args:  cobra.NoArgs,
		Short: "launch terraform HTTP backend",
		Long: `
This process is used as side-car to running terraform http backend. It will
proxy terraform state to TerraformState object.
		`,
		RunE: runBackend,
	}

	flags := cmd.Flags()

	// flags declared here should be cosistent with backendOpts structure
	flags.StringP("name", "n", "", "name of the terraform state object to use")
	flags.StringP("listen", "l", "localhost:8081", "listen port")
	_ = cmd.MarkFlagRequired("name")

	if err := viper.BindPFlags(flags); err != nil {
		panic(err)
	}
	return cmd
}

func runBackend(_ *cobra.Command, args []string) error {
	var opts backendOpts

	if err := viper.Unmarshal(&opts); err != nil {
		return err
	}

	return httpbackend.ListenAndServe(opts.Name, opts.Listen, opts.Debug)
}
