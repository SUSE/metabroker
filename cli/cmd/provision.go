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
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	client "github.com/SUSE/metabroker/apis/generated/clientset/versioned"
	servicebrokerv1alpha1 "github.com/SUSE/metabroker/apis/servicebroker/v1alpha1"
	"github.com/SUSE/metabroker/helm"
)

// NewDefaultProvisionCmd creates a new provision command with the default dependencies.
func NewDefaultProvisionCmd() *cobra.Command {
	return NewProvisionCmd(os.Stdout)
}

// NewProvisionCmd creates a new provision command.
func NewProvisionCmd(
	stdout io.Writer,
) *cobra.Command {
	var namespace string
	var values []string
	var timeout string

	cmd := &cobra.Command{
		Use:  "provision [INSTANCE NAME] [PLAN NAME]",
		Args: cobra.ExactArgs(2),
		// TODO: a better description.
		Short: "Creates an Instance object.",
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceName := args[0]
			planName := args[1]
			kubeconfigPath := viper.GetString("kubeconfig")

			valuesMap := make(map[string]interface{})
			for _, val := range values {
				f, err := os.Open(val)
				if err != nil {
					return fmt.Errorf("failed to provision: %w", err)
				}
				b, err := ioutil.ReadAll(f)
				if err != nil {
					return fmt.Errorf("failed to provision: %w", err)
				}
				v := make(map[string]interface{})
				if err := yaml.Unmarshal(b, &v); err != nil {
					return fmt.Errorf("failed to provision: %w", err)
				}
				valuesMap = helm.MergeMaps(valuesMap, v)
			}

			// TODO: add an interactive mode based on the plan JSON schema. A possible solution
			// would be to have an --interactive flag. It should not ask for values passed via the
			// --values flag, instead, it should merge the values with the map above. To accomplish
			// this securely, the JSON schema should be separated from the Plan and its spec should
			// reference another CRD specific to the JSON schema. This is important in order to have
			// the RBAC be efficient, otherwise to get the schema here would require full access to
			// reading the Plan, which should not be ideal in every case.

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to provision: %w", err)
			}
			clientset, err := client.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to provision: %w", err)
			}

			valuesYAMLBytes, err := yaml.Marshal(valuesMap)
			if err != nil {
				return fmt.Errorf("failed to provision: %w", err)
			}

			instance := &servicebrokerv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceName,
					Namespace: namespace,
				},
				Spec: servicebrokerv1alpha1.InstanceSpec{
					Plan:   planName,
					Values: string(valuesYAMLBytes),
				},
			}

			timeoutDuration, err := time.ParseDuration(timeout)
			if err != nil {
				return fmt.Errorf("failed to provision: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
			defer cancel()

			_, err = clientset.
				ServicebrokerV1alpha1().
				Instances(namespace).
				Create(ctx, instance, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to provision: %w", err)
			}

			fmt.Fprintf(
				stdout,
				"Provisioning instance %q in the %q namespace.\n",
				instanceName, namespace,
			)

			return nil
		},
	}

	cmd.LocalFlags().StringVarP(&namespace, "namespace", "n", "default", "The target namespace where the Instance will be created.")
	cmd.LocalFlags().StringSliceVarP(&values, "values", "f", []string{}, "(repeated) The values YAML files to be used for provisioning.")
	cmd.LocalFlags().StringVar(&timeout, "timeout", "5m0s", "The time to wait for the Instance to be created.")
	// TODO: add a --wait flag that watches the Instance status until it's ready.

	return cmd
}
