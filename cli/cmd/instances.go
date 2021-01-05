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

// NewDefaultInstancesCmd creates a new instances command with the default dependencies.
func NewDefaultInstancesCmd() *cobra.Command {
	return NewInstancesCmd(os.Stdout)
}

// NewInstancesCmd creates a new instances command.
func NewInstancesCmd(
	stdout io.Writer,
) *cobra.Command {
	var namespace string
	var timeout string
	var noHeaders bool

	cmd := &cobra.Command{
		Use:  "instances",
		Args: cobra.ExactArgs(0),
		// TODO: a better description.
		Short: "Lists the Instances on a given namespace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeconfigPath := viper.GetString("kubeconfig")

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to get instances: %w", err)
			}
			clientset, err := client.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to get instances: %w", err)
			}

			timeoutDuration, err := time.ParseDuration(timeout)
			if err != nil {
				return fmt.Errorf("failed to get instances: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
			defer cancel()

			instances, err := clientset.
				ServicebrokerV1alpha1().
				Instances(namespace).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("failed to get instances: %w", err)
			}

			if len(instances.Items) > 0 {
				w := &tabwriter.Writer{}
				w.Init(stdout, 8, 8, 0, '\t', 0)
				defer w.Flush()

				if !noHeaders {
					fmt.Fprintf(w, "%s  \t%s\n", "INSTANCE", "PLAN")
				}

				for _, instance := range instances.Items {
					fmt.Fprintf(w, "%s  \t%s\n", instance.Name, instance.Spec.Plan)
				}
			} else {
				fmt.Fprintf(stdout, "No instances found in the %q namespace.\n", namespace)
			}

			return nil
		},
	}

	cmd.LocalFlags().StringVarP(&namespace, "namespace", "n", "default", "The target namespace where the Instances to be listed were created.")
	cmd.LocalFlags().StringVar(&timeout, "timeout", "10s", "The time to wait for the Instances to be listed.")
	cmd.LocalFlags().BoolVar(&noHeaders, "no-headers", false, "Don't print headers (default print headers).")

	return cmd
}
