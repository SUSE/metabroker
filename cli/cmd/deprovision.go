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
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	client "github.com/SUSE/metabroker/apis/generated/clientset/versioned"
)

// NewDefaultDeprovisionCmd creates a new deprovision command with the default dependencies.
func NewDefaultDeprovisionCmd() *cobra.Command {
	return NewDeprovisionCmd(os.Stdout)
}

// NewDeprovisionCmd creates a new deprovision command.
func NewDeprovisionCmd(
	stdout io.Writer,
) *cobra.Command {
	var namespace string
	var timeout string

	cmd := &cobra.Command{
		Use:  "deprovision [INSTANCE NAME]",
		Args: cobra.ExactArgs(1),
		// TODO: a better description.
		Short: "Deletes an Instance object.",
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceName := args[0]
			kubeconfigPath := viper.GetString("kubeconfig")

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to deprovision: %w", err)
			}
			clientset, err := client.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to deprovision: %w", err)
			}

			timeoutDuration, err := time.ParseDuration(timeout)
			if err != nil {
				return fmt.Errorf("failed to deprovision: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
			defer cancel()

			err = clientset.
				ServicebrokerV1alpha1().
				Instances(namespace).
				Delete(ctx, instanceName, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed to deprovision: %w", err)
			}

			fmt.Fprintf(
				stdout,
				"Deprovisioning instance %q in the %q namespace.\n",
				instanceName, namespace,
			)

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "The target namespace where the Instance will be deleted.")
	cmd.Flags().StringVar(&timeout, "timeout", "1m0s", "The time to wait for the Instance to be deleted.")

	return cmd
}
