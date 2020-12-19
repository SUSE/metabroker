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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SUSE/metabroker/apis/stringutil"
)

const credentialUnbindingFinalizer = "unbinding.credentials.servicebroker.metabroker.suse.com"

// +genclient
// +kubebuilder:object:root=true

// Credential is the top-level Schema for the Credential resource API.
type Credential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// A namespaced reference to an existing Instance the credential is linked to.
	// required
	InstanceRef corev1.ObjectReference `json:"instanceRef"`
	// A reference to a secret in the same namespace as Instance. The secret must not be created
	// beforehand. If the secret happens to already exist, the Credential creation process fails.
	// required
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
	// The observed state of the Credential.
	Status CredentialStatus `json:"status,omitempty"`
}

// IsDeleting returns whether the Credential was marked for deletion but was not deleted yet. This
// is useful for checking when finalizers are present.
func (credential *Credential) IsDeleting() bool {
	return !credential.ObjectMeta.DeletionTimestamp.IsZero()
}

// HasUnbindingFinalizer returns whether the Credential has the unbinding finalizer.
func (credential *Credential) HasUnbindingFinalizer() bool {
	return stringutil.Contains(credential.ObjectMeta.Finalizers, credentialUnbindingFinalizer)
}

// AddUnbindingFinalizer appends the unbinding finalizer to the Credential finalizers.
func (credential *Credential) AddUnbindingFinalizer() {
	credential.ObjectMeta.Finalizers = append(credential.ObjectMeta.Finalizers, credentialUnbindingFinalizer)
}

// RemoveUnbindingFinalizer removes the unbinding finalizer from the Credential finalizers.
func (credential *Credential) RemoveUnbindingFinalizer() {
	credential.ObjectMeta.Finalizers = stringutil.Remove(
		credential.ObjectMeta.Finalizers,
		credentialUnbindingFinalizer,
	)
}

// +kubebuilder:object:root=true

// CredentialList contains a list of Credential.
type CredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Credential `json:"items"`
}

// CredentialStatus defines the observed state of Credential.
type CredentialStatus struct {
	// TODO: implement.
}

func init() {
	SchemeBuilder.Register(&Credential{}, &CredentialList{})
}
