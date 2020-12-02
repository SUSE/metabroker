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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebrokerv1alpha1 "github.com/SUSE/metabroker/operator/api/v1alpha1"
)

// InstanceReconciler implements the Reconcile method for the Instance resource.
type InstanceReconciler struct {
	client.Client

	log                  logr.Logger
	scheme               *runtime.Scheme
	metabrokerName       string
	provisioningPodImage string
}

// NewInstanceReconciler constructs a new InstanceReconciler.
func NewInstanceReconciler(metabrokerName, provisioningPodImage string) *InstanceReconciler {
	return &InstanceReconciler{
		metabrokerName:       metabrokerName,
		provisioningPodImage: provisioningPodImage,
	}
}

// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=instances/status,verbs=get;update;patch

const instanceReconcileTimeout = time.Second * 10

// Reconcile reconciles an Instance resource.
func (r *InstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), instanceReconcileTimeout)
	defer cancel()

	log := r.log.WithValues("instance", req.NamespacedName)

	releaseReq := ReleaseRequest{req, r.metabrokerName}

	instance := &servicebrokerv1alpha1.Instance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// The instance no longer exists; run any deprovisioning steps necessary.
			return r.runDeprovisioningPod(ctx, DeprovisioningRequest{releaseReq})
		}
		return ctrl.Result{}, err
	}

	planNamespacedName := req.NamespacedName
	planNamespacedName.Name = instance.Spec.Plan
	plan := &servicebrokerv1alpha1.Plan{}
	if err := r.Get(ctx, planNamespacedName, plan); err != nil {
		return ctrl.Result{}, err
	}

	instanceNeedsUpdate := false

	if len(instance.OwnerReferences) == 0 {
		if err := ctrl.SetControllerReference(plan, instance, r.scheme); err != nil {
			return ctrl.Result{}, nil
		}
		instanceNeedsUpdate = true
	}

	if instance.Spec.ID == "" {
		id := uuid.Must(uuid.NewUUID()) // UUID v1
		instance.Spec.ID = id.String()
		instanceNeedsUpdate = true
	}

	if instance.Spec.ValidateValues == nil {
		validateValues := true
		instance.Spec.ValidateValues = &validateValues
		instanceNeedsUpdate = true
	}

	if instanceNeedsUpdate {
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// TODO: update the status with validated so the controller doesn't keep performing the
	// validation on requeues.
	if *instance.Spec.ValidateValues {
		schemaJSON, err := yaml.YAMLToJSON([]byte(plan.Spec.Provisioning.Values.Schema))
		if err != nil {
			log.Error(err, "failed to validate values")
			// TODO: update status with invalid values.
			return ctrl.Result{}, nil
		}
		schema := gojsonschema.NewBytesLoader(schemaJSON)
		valuesJSON, err := yaml.YAMLToJSON([]byte(instance.Spec.Values))
		if err != nil {
			log.Error(err, "failed to validate values")
			// TODO: update status with invalid values.
			return ctrl.Result{}, nil
		}
		values := gojsonschema.NewBytesLoader(valuesJSON)
		result, err := gojsonschema.Validate(schema, values)
		if err != nil {
			log.Error(err, "failed to validate values")
			// TODO: update status with invalid values.
			return ctrl.Result{}, nil
		}
		if !result.Valid() {
			log.Error(err, "failed to validate values")
			// TODO: update status with invalid values. Including specific errors returned from
			// result.Errors().
			return ctrl.Result{}, nil
		}
	}

	valuesSecretName := fmt.Sprintf("metabroker-%s-values", instance.Name)
	if created, err := r.valuesSecret(ctx, instance, valuesSecretName, instance.Namespace); err != nil {
		return ctrl.Result{}, err
	} else if created {
		return ctrl.Result{Requeue: true}, nil
	}

	// TODO: determine if the provisioning pod should run again for a Helm upgrade or not.

	return r.runProvisioningPod(ctx, ProvisioningRequest{releaseReq}, instance, plan, valuesSecretName)
}

// valuesSecret creates a Secret containing the values.yaml content if it doesn't exist yet.
// The first return parameter represents whether the Secret was created or not.
func (r *InstanceReconciler) valuesSecret(
	ctx context.Context,
	instance *servicebrokerv1alpha1.Instance,
	valuesSecretName string,
	namespace string,
) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      valuesSecretName,
		Namespace: namespace,
	}
	current := &corev1.Secret{}
	if err := r.Get(ctx, namespacedName, current); err != nil {
		if !errors.IsNotFound(err) {
			return false, fmt.Errorf("failed to process values secret: %w", err)
		}
		desired := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namespacedName.Name,
				Namespace: namespacedName.Namespace,
				// TODO: add proper labels.
			},
			Data: map[string][]byte{"values.yaml": []byte(instance.Spec.Values)},
		}
		if err := ctrl.SetControllerReference(instance, desired, r.scheme); err != nil {
			return false, fmt.Errorf("failed to process values secret: %w", err)
		}
		if err := r.Create(ctx, desired); err != nil {
			return false, fmt.Errorf("failed to process values secret: %w", err)
		}
		return true, nil
	}
	return false, nil
}

// runProvisioningPod runs the provisioning pod. It creates the pod and its dependencies, ensuring
// that upon successful completion, all the created resources are deleted.
func (r *InstanceReconciler) runProvisioningPod(
	ctx context.Context,
	req ProvisioningRequest,
	instance *servicebrokerv1alpha1.Instance,
	plan *servicebrokerv1alpha1.Plan,
	valuesSecretName string,
) (ctrl.Result, error) {
	namespacedName := types.NamespacedName{
		Name:      req.Name(),
		Namespace: req.Namespace,
	}

	currentServiceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, namespacedName, currentServiceAccount); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		desiredServiceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name(),
				Namespace: req.Namespace,
				// TODO: add proper labels.
			},
		}
		if err := ctrl.SetControllerReference(instance, desiredServiceAccount, r.scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		if err := r.Create(ctx, desiredServiceAccount); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
	}

	desiredRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name(),
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      req.Name(),
			Namespace: req.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     "cluster-admin",
		},
	}
	if err := ctrl.SetControllerReference(instance, desiredRoleBinding, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
	}

	currentRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Get(ctx, namespacedName, currentRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		if err := r.Create(ctx, desiredRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
	}

	if !equality.Semantic.DeepDerivative(desiredRoleBinding.RoleRef, currentRoleBinding.RoleRef) {
		if err := r.Update(ctx, desiredRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
	}

	desiredPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name(),
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: req.Name(),
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "provisioning",
				Image:           r.provisioningPodImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/bin/catatonit", "--"},
				Args:            []string{"/bin/bash", "-c", provisioningScript},
				Env: []corev1.EnvVar{
					{Name: "NAME", Value: req.ReleaseName()},
					{Name: "CHART", Value: plan.Spec.Provisioning.Chart.URL},
					{Name: "NAMESPACE", Value: instance.Namespace},
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "values",
					ReadOnly:  true,
					MountPath: "/etc/metabroker-provisioning/",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "values",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: valuesSecretName},
				},
			}},
		},
	}
	if err := ctrl.SetControllerReference(instance, desiredPod, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
	}

	currentPod := &corev1.Pod{}
	if err := r.Get(ctx, namespacedName, currentPod); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		if err := r.Create(ctx, desiredPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredPod.Spec, currentPod.Spec) {
		if err := r.Update(ctx, desiredPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if currentPod.Status.Phase == corev1.PodSucceeded || currentPod.Status.Phase == corev1.PodFailed {
		// As soon as the pod gets completed, delete it and its dependencies.
		if err := r.Delete(ctx, currentServiceAccount); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		if err := r.Delete(ctx, currentRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		if err := r.Delete(ctx, currentPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to provision: %w", err)
		}
		if currentPod.Status.Phase == corev1.PodFailed {
			// TODO: what should we do when the pod fails?
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

const provisioningScript = `#!/bin/bash

set -o errexit -o nounset

helm upgrade "${NAME}" "${CHART}" \
  --install \
  --atomic \
  --namespace "${NAMESPACE}" \
  --values "/etc/metabroker-provisioning/values.yaml"
`

// runDeprovisioningPod runs the deprovisioning pod. It creates the pod and its dependencies,
// ensuring that upon successful completion, all the created resources are deleted.
func (r *InstanceReconciler) runDeprovisioningPod(
	ctx context.Context,
	req DeprovisioningRequest,
) (ctrl.Result, error) {
	namespacedName := types.NamespacedName{
		Name:      req.Name(),
		Namespace: req.Namespace,
	}

	currentServiceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, namespacedName, currentServiceAccount); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		desiredServiceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name(),
				Namespace: req.Namespace,
				// TODO: add proper labels.
			},
		}
		if err := r.Create(ctx, desiredServiceAccount); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
	}

	desiredRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name(),
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      req.Name(),
			Namespace: req.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     "cluster-admin",
		},
	}

	currentRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Get(ctx, namespacedName, currentRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		if err := r.Create(ctx, desiredRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
	}

	if !equality.Semantic.DeepDerivative(desiredRoleBinding.RoleRef, currentRoleBinding.RoleRef) {
		if err := r.Update(ctx, desiredRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
	}

	desiredPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name(),
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: req.Name(),
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "deprovisioning",
				Image:           r.provisioningPodImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/bin/catatonit", "--"},
				Args:            []string{"/bin/bash", "-c", deprovisioningScript},
				Env: []corev1.EnvVar{
					{Name: "NAME", Value: req.ReleaseName()},
					{Name: "NAMESPACE", Value: req.Namespace},
				},
			}},
		},
	}

	currentPod := &corev1.Pod{}
	if err := r.Get(ctx, namespacedName, currentPod); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		if err := r.Create(ctx, desiredPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredPod.Spec, currentPod.Spec) {
		if err := r.Update(ctx, desiredPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if currentPod.Status.Phase == corev1.PodSucceeded || currentPod.Status.Phase == corev1.PodFailed {
		// As soon as the pod gets completed, delete it and its dependencies.
		if err := r.Delete(ctx, currentServiceAccount); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		if err := r.Delete(ctx, currentRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		if err := r.Delete(ctx, currentPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to deprovision: %w", err)
		}
		if currentPod.Status.Phase == corev1.PodFailed {
			// TODO: what should we do when the pod fails?
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

const deprovisioningScript = `#!/bin/bash

set -o errexit -o nounset

if helm status "${NAME}" --namespace "${NAMESPACE}"; then
  helm delete "${NAME}" \
	--namespace "${NAMESPACE}"
fi
`

// SetupWithManager configures the controller manager for the Instance resource.
func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.scheme = mgr.GetScheme()
	r.log = ctrl.Log.WithName("controllers").WithName("Instance")
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebrokerv1alpha1.Instance{}).
		Complete(r)
}

// ReleaseRequest wraps ctrl.Request for providing a method that returns a canonical Helm release
// name.
type ReleaseRequest struct {
	ctrl.Request

	prefix string
}

// ReleaseName returns the canonical Helm release name based on the request name. This is useful for
// keeping name consistency across the Metabroker implementation.
func (req ReleaseRequest) ReleaseName() string {
	return fmt.Sprintf("%s-%s", req.prefix, req.Name)
}

// ProvisioningRequest wraps ReleaseRequest for providing the name used for all the provisioning
// objects.
type ProvisioningRequest struct {
	ReleaseRequest
}

// Name returns the provisioning name.
func (req ProvisioningRequest) Name() string {
	return fmt.Sprintf("%s-provision", req.ReleaseName())
}

// DeprovisioningRequest wraps ReleaseRequest for providing the name used for all the deprovisioning
// objects.
type DeprovisioningRequest struct {
	ReleaseRequest
}

// Name returns the deprovisioning name.
func (req DeprovisioningRequest) Name() string {
	return fmt.Sprintf("%s-deprovision", req.ReleaseName())
}
