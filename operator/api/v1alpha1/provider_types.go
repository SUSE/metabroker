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

// ProviderSpec defines the Service Provider spec.
type ProviderSpec struct {
	// The version of the Provider.
	Version string `json:"version"`
	// Provider upgrade contraints.
	// +optional
	Upgrades UpgradesSpec `json:"upgrades,omitempty"`
}

// UpgradesSpec defines the validation for upgrades of Service Providers.
type UpgradesSpec struct {
	// This is a version constraint as defined by:
	// https://github.com/Masterminds/semver#checking-version-constraints
	From string `json:"from,omitempty"`
}

// ProviderStatus defines the observed state of Provider.
type ProviderStatus struct {
	// TODO: implement.
}

// +kubebuilder:object:root=true

// Provider is the Schema for the providers API.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The specification of the desired behaviour of the Provider.
	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderList contains a list of Provider.
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}
