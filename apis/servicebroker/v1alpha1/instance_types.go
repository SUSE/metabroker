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

const instanceDeprovisioningFinalizer = "deprovisioning.instances.servicebroker.metabroker.suse.com"

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
}

// InstanceStatus defines the observed state of Instance.
type InstanceStatus struct {
	// TODO: implement.
}

// +genclient
// +kubebuilder:object:root=true

// Instance is the top-level Schema for the Instance resource API.
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The specification of the desired behaviour of the Instance.
	Spec InstanceSpec `json:"spec,omitempty"`
	// The reference to the Helm installation that contains the Instance workload.
	// +optional
	HelmRef corev1.LocalObjectReference `json:"helmRef,omitempty"`
	// The reference to the secret containing the Helm installation values for the Instance.
	// +optional
	HelmValuesRef corev1.LocalObjectReference `json:"helmValuesRef,omitempty"`
	Status        InstanceStatus              `json:"status,omitempty"`
}

// IsDeleting returns whether the Instance was marked for deletion but was not deleted yet. This
// is useful for checking when finalizers are present.
func (instance *Instance) IsDeleting() bool {
	return !instance.ObjectMeta.DeletionTimestamp.IsZero()
}

// HasDeprovisioningFinalizer returns whether the Instance has the deprovisioning finalizer.
func (instance *Instance) HasDeprovisioningFinalizer() bool {
	return stringutil.Contains(instance.ObjectMeta.Finalizers, instanceDeprovisioningFinalizer)
}

// AddDeprovisioningFinalizer appends the deprovisioning finalizer to the Instance finalizers.
func (instance *Instance) AddDeprovisioningFinalizer() {
	instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, instanceDeprovisioningFinalizer)
}

// RemoveDeprovisioningFinalizer removes the deprovisioning finalizer from the Instance finalizers.
func (instance *Instance) RemoveDeprovisioningFinalizer() {
	instance.ObjectMeta.Finalizers = stringutil.Remove(
		instance.ObjectMeta.Finalizers,
		instanceDeprovisioningFinalizer,
	)
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
