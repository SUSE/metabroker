/*
Copyright 2020 SUSE

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PlanSpec defines the Service Plan spec.
type PlanSpec struct {
	// The name of the Offering this Plan belongs to.
	Offering string `json:"offering"`
	// A unique ID for the Plan to be used by OSBAPI. If not provided, a UUID v1 is auto-generated.
	// +optional
	ID string `json:"id,omitempty"`
	// A description for the Plan.
	// +optional
	Description string `json:"description,omitempty"`
	// The specification for how an Instance of the Plan should be provisioned.
	// TODO: pull the Provisioning out into its own CRD, so plans can safely be listed by
	// non-platform operators. Also, add a comment to the PlanSpec for future reference what
	// can/cannot be added to the spec.
	Provisioning PlanProvisioningSpec `json:"provisioning"`
	// The specification for how an Instance of the Plan should be bound.
	// TODO: pull the Binding out into its own CRD, so plans can safely be listed by
	// non-platform operators. Also, add a comment to the PlanSpec for future reference what
	// can/cannot be added to the spec.
	Binding PlanBindingSpec `json:"binding"`
}

// PlanProvisioningSpec defines the Provisioning spec for a Plan.
type PlanProvisioningSpec struct {
	// Chart contains what chart to be used to provision an Instance of the Plan.
	Chart PlanProvisioningChartSpec `json:"chart"`
	// Values contains the configuration for validating user-provided provisioning properties as
	// well as plan-specific default values and overrides.
	Values PlanProvisioningValuesSpec `json:"values"`
	// TODO: add a provisioning hook that has the capability of modifying the secret containing the
	// values.yaml. This can be separated into pre-install and pre-upgrade, as some charts require
	// the upgrade to contain values extracted from the previous installation.
}

// PlanProvisioningChartSpec defines the Chart spec for a Provisioning.
type PlanProvisioningChartSpec struct {
	// The URL for the Chart tarball.
	URL string `json:"url"`
	// The SHA-256 checksum for the Chart tarball.
	SHA256 string `json:"sha256"`
}

// PlanProvisioningValuesSpec defines the Values spec for a Provisioning.
type PlanProvisioningValuesSpec struct {
	// The JSON schema for validating user-provided properties that are passed to the Helm client as
	// values for installing a Chart. The schema definition can be written as YAML or JSON.
	Schema string `json:"schema"`
	// The default values used to override the Chart defaults. The user-provided values can still
	// override these values.
	// +optional
	Default string `json:"default,omitempty"`
	// The static values applied on top of all other values used to enforce plan-specific
	// configuration.
	// +optional
	Static string `json:"static,omitempty"`
}

// PlanBindingSpec defines the Binding spec for a Plan.
type PlanBindingSpec struct {
	// The specification of the desired behaviour of the binding credentials.
	Credentials PlanBindingCredentialsSpec `json:"credentials"`
}

// PlanBindingCredentialsSpec defines the Credentials spec for a Binding.
type PlanBindingCredentialsSpec struct {
	// The container specification for the logic of binding an Instance of the Plan. Instance
	// specifics are passed to the container process via environment variables and mounted files.
	// Environment variables:
	//   - METABROKER_INSTANCE_NAME: the name of the Instance being bound.
	//   - METABROKER_INSTANCE_HELM_NAME: the generated name for the Instance Helm installation.
	//   - METABROKER_CREDENTIAL_NAME: the name of the Credential that triggered the binding.
	//   - METABROKER_VALUES_FILE: a path to the values YAML file used in the Instance Helm
	//       installation.
	//   - METABROKER_HELM_OBJECTS_LIST_FILE: a path to the file containing a list of all recources
	//       directly created by the provisioning Helm installation.
	//   - METABROKER_OUTPUT: the name of the Kubernetes object to be patched to output the
	//       generated credentials in the format "secret/<name>".
	RunContainer PlanBindingCredentialsRunContainerSpec `json:"runContainer"`
}

// PlanBindingCredentialsRunContainerSpec defines the container spec for a Binding Credential.
type PlanBindingCredentialsRunContainerSpec struct {
	// The image repository, including the registry.
	Image string `json:"image"`
	// The entrypoint command used for the container.
	// +optional
	Command []string `json:"command,omitempty"`
	// The arguments passed to the entrypoint command.
	// +optional
	Args []string `json:"args,omitempty"`
}

// PlanStatus defines the observed state of Plan.
type PlanStatus struct {
	// TODO: implement.
}

// +genclient
// +kubebuilder:object:root=true

// Plan is the top-level Schema for the Plan resource API.
type Plan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The specification of the desired behaviour of the Plan.
	Spec   PlanSpec   `json:"spec,omitempty"`
	Status PlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PlanList contains a list of Plan.
type PlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Plan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Plan{}, &PlanList{})
}
