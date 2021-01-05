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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	client "github.com/SUSE/metabroker/apis/generated/clientset/versioned"
	servicebrokerv1alpha1 "github.com/SUSE/metabroker/apis/servicebroker/v1alpha1"
)

// NewDefaultBindCmd creates a new bind command with the default dependencies.
func NewDefaultBindCmd() *cobra.Command {
	return NewBindCmd(os.Stdout)
}

// NewBindCmd creates a new bind command.
func NewBindCmd(
	stdout io.Writer,
) *cobra.Command {
	var credentialNamespace string
	var instanceNamespace string
	var secretName string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:  "bind [INSTANCE NAME] [CREDENTIAL NAME]",
		Args: cobra.ExactArgs(2),
		// TODO: a better description.
		Short: "Creates a Credential object.",
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceName := args[0]
			credentialName := args[1]
			kubeconfigPath := viper.GetString("kubeconfig")

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to bind: %w", err)
			}
			clientset, err := client.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to bind: %w", err)
			}

			if secretName == "" {
				secretName = credentialName
			}

			credential := &servicebrokerv1alpha1.Credential{
				ObjectMeta: metav1.ObjectMeta{
					Name:      credentialName,
					Namespace: credentialNamespace,
				},
				InstanceRef: corev1.ObjectReference{
					Name:      instanceName,
					Namespace: instanceNamespace,
				},
				SecretRef: corev1.LocalObjectReference{
					Name: secretName,
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			_, err = clientset.
				ServicebrokerV1alpha1().
				Credentials(credentialNamespace).
				Create(ctx, credential, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to bind: %w", err)
			}

			fmt.Fprintf(
				stdout,
				"Binding instance %q in the %q namespace to the credential %q in the %q namespace.\n",
				instanceName, instanceNamespace, secretName, credentialNamespace,
			)

			return nil
		},
	}

	cmd.LocalFlags().StringVarP(&credentialNamespace, "namespace", "n", "default", "The target namespace where the Credential will be created.")
	cmd.LocalFlags().StringVar(&instanceNamespace, "instance-namespace", "default", "The target namespace where the Instance was created.")
	cmd.LocalFlags().StringVar(&secretName, "secret-name", "", "The secret name where the credentials should be created. The secret must not exist. If empty, a secret with the Credential name is created.")
	cmd.LocalFlags().DurationVar(&timeout, "timeout", time.Minute*3, "The time to wait for the Credential to be created.")

	return cmd
}
