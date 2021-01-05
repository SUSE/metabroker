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
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	client "github.com/SUSE/metabroker/apis/generated/clientset/versioned"
)

// NewDefaultCredentialsCmd creates a new credentials command with the default dependencies.
func NewDefaultCredentialsCmd() *cobra.Command {
	return NewCredentialsCmd(os.Stdout)
}

// NewCredentialsCmd creates a new credentials command.
func NewCredentialsCmd(
	stdout io.Writer,
) *cobra.Command {
	var namespace string
	var timeout time.Duration
	var noHeaders bool

	cmd := &cobra.Command{
		Use:  "credentials",
		Args: cobra.ExactArgs(0),
		// TODO: a better description.
		Short: "Lists the Credentials on a given namespace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeconfigPath := viper.GetString("kubeconfig")

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to get credentials: %w", err)
			}
			clientset, err := client.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to get credentials: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			credentials, err := clientset.
				ServicebrokerV1alpha1().
				Credentials(namespace).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("failed to get credentials: %w", err)
			}

			if len(credentials.Items) > 0 {
				w := &tabwriter.Writer{}
				w.Init(stdout, 8, 8, 0, '\t', 0)
				defer w.Flush()

				if !noHeaders {
					fmt.Fprintf(w, "%s  \t%s\n", "CREDENTIAL", "INSTANCE")
				}

				for _, credential := range credentials.Items {
					instance := credential.InstanceRef
					fmt.Fprintf(w, "%s  \t%s/%s\n", credential.Name, instance.Namespace, instance.Name)
				}
			} else {
				fmt.Fprintf(stdout, "No plans found in the %q namespace.\n", namespace)
			}

			return nil
		},
	}

	cmd.LocalFlags().StringVarP(&namespace, "namespace", "n", "default", "The target namespace where the Credentials to be listed were created.")
	cmd.LocalFlags().DurationVar(&timeout, "timeout", time.Second*10, "The time to wait for the Credentials to be listed.")
	cmd.LocalFlags().BoolVar(&noHeaders, "no-headers", false, "Don't print headers (default print headers).")

	return cmd
}
