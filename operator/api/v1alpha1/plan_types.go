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
	Offering string `json:"offering,omitempty"`

	// A unique ID for the Plan.
	ID string `json:"id,omitempty"`
	// A description for the Plan.
	Description string `json:"description,omitempty"`

	Provisioning PlanProvisioningSpec `json:"provisioning,omitempty"`
	Binding      PlanBindingSpec      `json:"binding,omitempty"`
}

// PlanProvisioningSpec defines the Provisioning spec for a Plan.
type PlanProvisioningSpec struct {
	Chart  PlanProvisioningChartSpec  `json:"chart,omitempty"`
	Values PlanProvisioningValuesSpec `json:"values,omitempty"`
}

// PlanProvisioningChartSpec defines the Chart spec for a Provisioning.
type PlanProvisioningChartSpec struct {
	URL string `json:"url,omitempty"`
}

// PlanProvisioningValuesSpec defines the Values spec for a Provisioning. It includes the JSON
// schema for validation, the default values that can be overriden by the user, and the static
// values that enforce plan-specific configuration.
type PlanProvisioningValuesSpec struct {
	Schema  string `json:"schema,omitempty"`
	Default string `json:"default,omitempty"`
	Static  string `json:"static,omitempty"`
}

// PlanBindingSpec defines the Binding spec for a Plan.
type PlanBindingSpec struct {
	Credentials PlanBindingCredentialsSpec `json:"credentials,omitempty"`
}

// PlanBindingCredentialsSpec defines the Credentials spec for a Binding.
type PlanBindingCredentialsSpec struct {
	Script PlanBindingCredentialsScriptSpec `json:"script,omitempty"`
}

// PlanBindingCredentialsScriptSpec defines the Script spec for a Binding Credential.
type PlanBindingCredentialsScriptSpec struct {
	Implementation string `json:"implementation,omitempty"`
	Type           string `json:"type,omitempty"`
}

// PlanStatus defines the observed state of Plan.
type PlanStatus struct {
	// TODO: implement.
}

// +kubebuilder:object:root=true

// Plan is the Schema for the plans API.
type Plan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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
