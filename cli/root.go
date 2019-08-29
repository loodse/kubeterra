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
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// globalOptions is a struct to embedd to other "opts" structures for
// viper.Unmarshal
type globalOptions struct {
	Verbose bool
	Debug   bool
}

// Execute is the root command entry function
func Execute() {
	rootCmd := newRoot()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func newRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeterra",
		Short: "TODO",
		Long: `
Terraform controllers manager
		`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}

	gopts := globalOptions{}
	flags := cmd.PersistentFlags()

	// flags declared here should be cosistent with rootOpts structure
	flags.BoolVarP(&gopts.Verbose, "verbose", "v", false, "verbose output")
	flags.BoolVarP(&gopts.Debug, "debug", "d", false, "development mode")

	cmd.AddCommand(
		managerCmd(&gopts),
		backendCmd(&gopts),
	)

	return cmd
}
