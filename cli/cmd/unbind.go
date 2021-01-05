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

// NewDefaultUnbindCmd creates a new unbind command with the default dependencies.
func NewDefaultUnbindCmd() *cobra.Command {
	return NewUnbindCmd(os.Stdout)
}

// NewUnbindCmd creates a new unbind command.
func NewUnbindCmd(
	stdout io.Writer,
) *cobra.Command {
	var namespace string
	var timeout string

	cmd := &cobra.Command{
		Use:  "unbind [CREDENTIAL NAME]",
		Args: cobra.ExactArgs(1),
		// TODO: a better description.
		Short: "Deletes a Credential object.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			kubeconfigPath := viper.GetString("kubeconfig")

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to unbind: %w", err)
			}
			clientset, err := client.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to unbind: %w", err)
			}

			timeoutDuration, err := time.ParseDuration(timeout)
			if err != nil {
				return fmt.Errorf("failed to unbind: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
			defer cancel()

			credential, err := clientset.
				ServicebrokerV1alpha1().
				Credentials(namespace).
				Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to unbind: %w", err)
			}

			err = clientset.
				ServicebrokerV1alpha1().
				Credentials(namespace).
				Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed to unbind: %w", err)
			}

			instance := credential.InstanceRef
			secret := credential.SecretRef
			fmt.Fprintf(
				stdout,
				"Unbinding instance %q in the %q namespace from the credential %q in the %q namespace.\n",
				instance.Name, instance.Namespace, secret.Name, namespace,
			)

			return nil
		},
	}

	cmd.LocalFlags().StringVarP(&namespace, "namespace", "n", "default", "The target namespace where the Credential will be deleted.")
	cmd.LocalFlags().StringVar(&timeout, "timeout", "3m0s", "The time to wait for the Credential to be deleted.")

	return cmd
}
