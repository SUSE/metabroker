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

// InstanceSpec defines the Service Instance spec.
type InstanceSpec struct {
	// The name of the Plan this Instance should be provisioned with.
	Plan string `json:"plan"`
	// A unique ID for the Instance to be used by OSBAPI. If not provided, a UUID v1 is
	// auto-generated.
	// +optional
	ID string `json:"id,omitempty"`
	// The values used for provisioning the Instance.
	Values string `json:"values"`
	// Whether to validate the Values with the Plan's JSON schema or not. When an Instance is
	// created via Metabroker's OSBAPI, this should be omitted as the OSBAPI implementation already
	// performs this validation. If omitted, this field defaults to true.
	// +optional
	ValidateValues *bool `json:"validateValues,omitempty"`
}

// InstanceStatus defines the observed state of Instance.
type InstanceStatus struct {
	// TODO: implement.
}

// +kubebuilder:object:root=true

// Instance is the top-level Schema for the Instance resource API.
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec,omitempty"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance.
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
