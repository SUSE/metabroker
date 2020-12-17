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
	stderrors "errors"
	"fmt"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebrokerv1alpha1 "github.com/SUSE/metabroker/api/v1alpha1"
	"github.com/SUSE/metabroker/helm"
)

// InstanceReconciler implements the Reconcile method for the Instance resource.
type InstanceReconciler struct {
	client.Client

	log                  logr.Logger
	scheme               *runtime.Scheme
	helm                 helm.Client
	metabrokerName       string
	provisioningPodImage string
}

// NewInstanceReconciler constructs a new InstanceReconciler.
func NewInstanceReconciler(
	helm helm.Client,
	metabrokerName string,
	provisioningPodImage string,
) *InstanceReconciler {
	return &InstanceReconciler{
		helm:                 helm,
		metabrokerName:       metabrokerName,
		provisioningPodImage: provisioningPodImage,
	}
}

// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=instances/status,verbs=get;update;patch

const (
	instanceReconcileTimeout = 30 * time.Second
	helmInstallTimeout       = 5 * time.Minute
	helmUninstallTimeout     = 3 * time.Minute
)

// Reconcile reconciles an Instance resource.
func (r *InstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), instanceReconcileTimeout)
	defer cancel()

	log := r.log.WithValues("instance", req.NamespacedName)

	releaseReq := ReleaseRequest{Request: req, prefix: r.metabrokerName}

	instance := &servicebrokerv1alpha1.Instance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Instance deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if instance.IsDeleting() {
		if instance.HasDeprovisioningFinalizer() {
			log.Info("Instance being deleted; deprovisioning...")
			uninstallOpts := helm.UninstallOpts{
				Namespace: helm.NamespaceOpt(releaseReq.Namespace),
				Timeout:   helmUninstallTimeout,
			}
			if err := r.helm.Uninstall(instance.HelmRef.Name, uninstallOpts); err != nil {
				return ctrl.Result{}, err
			}

			instance.RemoveDeprovisioningFinalizer()
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !instance.HasDeprovisioningFinalizer() {
		instance.AddDeprovisioningFinalizer()
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
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
			return ctrl.Result{}, err
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

	if instance.HelmRef.Name == "" {
		instance.HelmRef.Name = releaseReq.ReleaseName()
		instanceNeedsUpdate = true
	}

	if instance.HelmValuesRef.Name == "" {
		instance.HelmValuesRef.Name = fmt.Sprintf("%s-values-yaml", releaseReq.ReleaseName())
		instanceNeedsUpdate = true
	}

	if instanceNeedsUpdate {
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
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

	// TODO: determine if Helm upgrade should run again or not depending on the status and event.

	rel, err := r.getOrInstall(instance, plan, releaseReq)
	if err != nil {
		return ctrl.Result{}, err
	}

	releaseValues, err := yaml.Marshal(helm.MergeMaps(rel.Chart.Values, rel.Config))
	if err != nil {
		return ctrl.Result{}, err
	}
	desiredValuesSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.HelmValuesRef.Name,
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Data: map[string][]byte{"values.yaml": releaseValues},
	}
	if err := ctrl.SetControllerReference(instance, desiredValuesSecret, r.scheme); err != nil {
		return ctrl.Result{}, err
	}

	currentValuesSecret := &corev1.Secret{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredValuesSecret.Name,
			Namespace: desiredValuesSecret.Namespace,
		},
		currentValuesSecret,
	); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredValuesSecret); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredValuesSecret.Data, currentValuesSecret.Data) {
		if err := r.Update(ctx, desiredValuesSecret); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	ready, err := r.helm.IsReady(rel)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: update status of Instance.

	if !ready {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) getOrInstall(
	instance *servicebrokerv1alpha1.Instance,
	plan *servicebrokerv1alpha1.Plan,
	releaseReq ReleaseRequest,
) (*release.Release, error) {
	releaseName := releaseReq.ReleaseName()
	namespace := helm.NamespaceOpt(releaseReq.Namespace)
	getOpts := helm.GetOpts{Namespace: namespace}
	rel, err := r.helm.Get(releaseName, getOpts)
	if err != nil {
		if stderrors.Is(err, driver.ErrReleaseNotFound) {
			values, err := mergeValues(instance, plan)
			if err != nil {
				// TODO: log that constructing the values failed. This is a problem with the plan config
				// (the combination of user-provided values validation, the default values and the static
				// values). Also, update the Instance status with the reasons for failing.
				return nil, nil
			}
			chartInfo := helm.ChartInfo{
				URL:       plan.Spec.Provisioning.Chart.URL,
				SHA256Sum: plan.Spec.Provisioning.Chart.SHA256,
			}
			installOpts := helm.InstallOpts{
				Atomic:      false,
				Description: plan.Spec.Description,
				Namespace:   namespace,
				Timeout:     helmInstallTimeout,
				Values:      values,
				Wait:        false,
			}
			rel, err := r.helm.Install(releaseName, chartInfo, installOpts)
			if err != nil {
				return nil, err
			}
			return rel, nil
		}
		return nil, err
	}
	return rel, nil
}

// mergeValues merges the user-provided values and the default and static values provided by the
// plan. The static values take precedence over the user-provided values, which in turn takes
// precedence over the default values.
func mergeValues(
	instance *servicebrokerv1alpha1.Instance,
	plan *servicebrokerv1alpha1.Plan,
) (map[string]interface{}, error) {
	userValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(instance.Spec.Values), &userValues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user-provided values: %w", err)
	}
	defaultValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(plan.Spec.Provisioning.Values.Default), &defaultValues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default plan values: %w", err)
	}
	staticValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(plan.Spec.Provisioning.Values.Static), &staticValues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal static plan values: %w", err)
	}
	values := make(map[string]interface{})
	values = helm.MergeMaps(values, defaultValues)
	values = helm.MergeMaps(values, userValues)
	values = helm.MergeMaps(values, staticValues)
	return values, nil
}

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
