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

// OfferingSpec defines the Service Offering spec.
type OfferingSpec struct {
	// The name of the Provider this Offering belongs to. The Offering controller sets ownership to
	// the Provider specified here.
	Provider string `json:"provider"`
	// A unique ID for the Plan to be used by OSBAPI. If not provided, a UUID v1 is auto-generated.
	// +optional
	ID string `json:"id,omitempty"`
	// A description for the Offering.
	// +optional
	Description string `json:"description,omitempty"`
	// Whether the Plans linked to this Offering are bindable or not.
	// +optional
	Bindable bool `json:"bindable,omitempty"`
}

// OfferingStatus defines the observed state of Offering.
type OfferingStatus struct {
	// TODO: implement.
}

// +kubebuilder:object:root=true

// Offering is the top-level Schema for the Offering resource API.
type Offering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The specification of the desired behaviour of the Offering.
	Spec   OfferingSpec   `json:"spec,omitempty"`
	Status OfferingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OfferingList contains a list of Offering.
type OfferingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Offering `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Offering{}, &OfferingList{})
}
