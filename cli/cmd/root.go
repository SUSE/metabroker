/*
Copyright 2021 SUSE

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

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/util/homedir"
)

// NewDefaultRootCmd creates a new root command as entrypoint for all subcommands with the default
// subcommands constructors.
func NewDefaultRootCmd() *cobra.Command {
	return NewRootCmd(
		NewDefaultMarketplaceCmd(),
		NewDefaultProvisionCmd(),
		NewDefaultDeprovisionCmd(),
		NewDefaultInstancesCmd(),
		NewDefaultBindCmd(),
		NewDefaultUnbindCmd(),
		NewDefaultCredentialsCmd(),
	)
}

// NewRootCmd creates a new root command as entrypoint for all subcommands.
func NewRootCmd(
	marketplaceCmd *cobra.Command,
	provisionCmd *cobra.Command,
	deprovisionCmd *cobra.Command,
	instancesCmd *cobra.Command,
	bindCmd *cobra.Command,
	unbindCmd *cobra.Command,
	credentialsCmd *cobra.Command,
) *cobra.Command {
	var kubeconfig string

	cmd := &cobra.Command{
		Use: "metabrokerctl",
		// TODO: a better description.
		Short:        "metabrokerctl manages Metabroker.",
		SilenceUsage: true,
	}

	const kubeconfigFlag = "kubeconfig"
	const kubeconfigFlagDesc = "The absolute path to the kubeconfig file."
	if home := homedir.HomeDir(); home != "" {
		cmd.PersistentFlags().StringVar(&kubeconfig, kubeconfigFlag, filepath.Join(home, ".kube", "config"), fmt.Sprintf("(optional) %s", kubeconfigFlagDesc))
	} else {
		cmd.PersistentFlags().StringVar(&kubeconfig, kubeconfigFlag, "", kubeconfigFlagDesc)
	}
	viper.BindPFlag(kubeconfigFlag, cmd.PersistentFlags().Lookup(kubeconfigFlag))

	cmd.AddCommand(marketplaceCmd)
	cmd.AddCommand(provisionCmd)
	cmd.AddCommand(deprovisionCmd)
	cmd.AddCommand(instancesCmd)
	cmd.AddCommand(bindCmd)
	cmd.AddCommand(unbindCmd)
	cmd.AddCommand(credentialsCmd)

	return cmd
}
