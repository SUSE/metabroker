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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	discoveryclient "k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebrokerv1alpha1 "github.com/SUSE/metabroker/api/v1alpha1"
	"github.com/SUSE/metabroker/helm"
)

// CredentialReconciler implements the Reconcile method for the Credential resource.
type CredentialReconciler struct {
	client.Client
	*discoveryclient.DiscoveryClient

	log    logr.Logger
	scheme *runtime.Scheme
	helm   helm.Client

	resourceCache map[kindWithGroup]metav1.APIResource
}

// NewCredentialReconciler constructs a new CredentialReconciler.
func NewCredentialReconciler(helm helm.Client) *CredentialReconciler {
	return &CredentialReconciler{
		helm:          helm,
		resourceCache: make(map[kindWithGroup]metav1.APIResource),
	}
}

// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=credentials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=credentials/status,verbs=get;update;patch

const credentialReconcileTimeout = 10 * time.Second

// Reconcile reconciles a Credential resource.
func (r *CredentialReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), credentialReconcileTimeout)
	defer cancel()

	log := r.log.WithValues("credential", req.NamespacedName)

	credential := &servicebrokerv1alpha1.Credential{}
	if err := r.Get(ctx, req.NamespacedName, credential); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Credential deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if credential.IsDeleting() {
		if credential.HasUnbindingFinalizer() {
			log.Info("Credential being deleted; unbinding...")
			// TODO: The credential is being deleted; run any unbinding steps necessary.

			credential.RemoveUnbindingFinalizer()
			if err := r.Update(ctx, credential); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !credential.HasUnbindingFinalizer() {
		credential.AddUnbindingFinalizer()
		if err := r.Update(ctx, credential); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	instance := &servicebrokerv1alpha1.Instance{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      credential.InstanceRef.Name,
			Namespace: credential.InstanceRef.Namespace,
		},
		instance,
	); err != nil {
		return ctrl.Result{}, err
	}

	if len(credential.OwnerReferences) == 0 {
		if err := ctrl.SetControllerReference(instance, credential, r.scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Update(ctx, credential); err != nil {
			return ctrl.Result{}, err
		}
		// TODO: make this a constant.
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	secret, err := r.getOrCreateSecret(ctx, &req, credential)
	if err != nil {
		return ctrl.Result{}, err
	}
	if secret == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	plan := &servicebrokerv1alpha1.Plan{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      instance.Spec.Plan,
			Namespace: req.Namespace,
		},
		plan,
	); err != nil {
		return ctrl.Result{}, err
	}

	return r.runBindingPod(ctx, &req, credential, instance, plan, secret.Name)
}

// getOrCreateSecret returns or creates the existing secret referenced by the credential. When the
// secret and the error returning values are nil, it means the secret was created. A requeue should
// be issued for the next reconcile loop to get the current secret from the API server.
func (r *CredentialReconciler) getOrCreateSecret(
	ctx context.Context,
	req *ctrl.Request,
	credential *servicebrokerv1alpha1.Credential,
) (*corev1.Secret, error) {
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      credential.SecretRef.Name,
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Data: map[string][]byte{},
	}
	if err := ctrl.SetControllerReference(credential, desired, r.scheme); err != nil {
		return nil, err
	}
	current := &corev1.Secret{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      credential.SecretRef.Name,
			Namespace: req.Namespace,
		},
		current,
	); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		if err := r.Create(ctx, desired); err != nil {
			return nil, err
		}
		return nil, nil
	}
	// TODO: if the current secret is not owned by the credential, fail.
	return current, nil
}

func (r *CredentialReconciler) runBindingPod(
	ctx context.Context,
	req *ctrl.Request,
	credential *servicebrokerv1alpha1.Credential,
	instance *servicebrokerv1alpha1.Instance,
	plan *servicebrokerv1alpha1.Plan,
	credentialsSecretName string,
) (ctrl.Result, error) {
	desiredServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
	}
	if err := ctrl.SetControllerReference(credential, desiredServiceAccount, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	currentServiceAccount := &corev1.ServiceAccount{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredServiceAccount.Name,
			Namespace: desiredServiceAccount.Namespace,
		},
		currentServiceAccount,
	); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Create(ctx, desiredServiceAccount); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	helmReleaseGetOpts := helm.GetOpts{Namespace: helm.NamespaceOpt(req.Namespace)}
	helmRelease, err := r.helm.Get(
		instance.HelmRef.Name,
		helmReleaseGetOpts,
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	helmResources, err := r.helm.ListResources(helmRelease)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	helmObjectsPolicyRules := make([]rbacv1.PolicyRule, len(helmResources))
	for i, res := range helmResources {
		obj := helm.AsVersioned(res)
		gv := obj.GetObjectKind().GroupVersionKind()
		kwg := kindWithGroup{kind: gv.Kind, group: gv.Group}
		resource, err := r.resourceForKind(kwg)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind; could not find resource for kind %q: %w", kwg, err)
		}
		helmObjectsPolicyRules[i] = rbacv1.PolicyRule{
			APIGroups:     []string{gv.Group},
			Resources:     []string{resource.Name},
			ResourceNames: []string{res.Name},
			Verbs:         []string{"get"},
		}
	}

	desiredRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Rules: append(
			// A rule to allow patching the credential secret.
			[]rbacv1.PolicyRule{{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{credentialsSecretName},
				Verbs:         []string{"get", "patch"},
			}},
			// The rules to allow reading the Kubernetes objects the Helm release created.
			helmObjectsPolicyRules...,
		),
	}
	if err := ctrl.SetControllerReference(credential, desiredRole, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	currentRole := &rbacv1.Role{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredRole.Name,
			Namespace: desiredRole.Namespace,
		},
		currentRole,
	); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Create(ctx, desiredRole); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredRole.Rules, currentRole.Rules) {
		if err := r.Update(ctx, desiredRole); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	desiredRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      desiredServiceAccount.Name,
			Namespace: desiredServiceAccount.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     desiredRole.Name,
		},
	}
	if err := ctrl.SetControllerReference(credential, desiredRoleBinding, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	currentRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredRoleBinding.Name,
			Namespace: desiredRoleBinding.Namespace,
		},
		currentRoleBinding,
	); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Create(ctx, desiredRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredRoleBinding.RoleRef, currentRoleBinding.RoleRef) {
		if err := r.Update(ctx, desiredRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	helmObjectsList := make([]corev1.ObjectReference, len(helmResources))
	for i, res := range helmResources {
		helmObjectsList[i] = corev1.ObjectReference{
			Name:      res.ObjectName(),
			Namespace: res.Namespace,
		}
	}

	helmObjectsNamespacedNameListStr, err := yaml.Marshal(&helmObjectsList)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	desiredHelmObjectsListConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-helm-objects", req.Name),
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		BinaryData: map[string][]byte{
			"helm_objects_list.yaml": helmObjectsNamespacedNameListStr,
		},
	}
	if err := ctrl.SetControllerReference(credential, desiredHelmObjectsListConfigMap, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	currentHelmObjectsListConfigMap := &corev1.ConfigMap{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredHelmObjectsListConfigMap.Name,
			Namespace: desiredHelmObjectsListConfigMap.Namespace,
		},
		currentHelmObjectsListConfigMap,
	); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Create(ctx, desiredHelmObjectsListConfigMap); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredHelmObjectsListConfigMap.BinaryData, currentHelmObjectsListConfigMap.BinaryData) {
		if err := r.Update(ctx, desiredHelmObjectsListConfigMap); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	desiredPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			// TODO: add proper labels.
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: desiredServiceAccount.Name,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "binding",
				Image:           plan.Spec.Binding.Credentials.RunContainer.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         plan.Spec.Binding.Credentials.RunContainer.Command,
				Args:            plan.Spec.Binding.Credentials.RunContainer.Args,
				Env: []corev1.EnvVar{
					{
						Name:  "METABROKER_INSTANCE_NAME",
						Value: instance.Name,
					},
					{
						Name:  "METABROKER_INSTANCE_HELM_NAME",
						Value: instance.HelmRef.Name,
					},
					{
						Name:  "METABROKER_CREDENTIAL_NAME",
						Value: credential.Name,
					},
					{
						Name:  "METABROKER_VALUES_FILE",
						Value: "/etc/metabroker/values.yaml",
					},
					{
						Name:  "METABROKER_HELM_OBJECTS_LIST_FILE",
						Value: "/etc/metabroker/helm_objects_list.yaml",
					},
					{
						Name:  "METABROKER_OUTPUT",
						Value: fmt.Sprintf("secret/%s", credentialsSecretName),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "values",
						ReadOnly:  true,
						MountPath: "/etc/metabroker/values.yaml",
						SubPath:   "values.yaml",
					},
					{
						Name:      "helm-objects-list",
						ReadOnly:  true,
						MountPath: "/etc/metabroker/helm_objects_list.yaml",
						SubPath:   "helm_objects_list.yaml",
					},
				},
			}},
			Volumes: []corev1.Volume{
				{
					Name: "values",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: instance.HelmValuesRef.Name},
					},
				},
				{
					Name: "helm-objects-list",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: currentHelmObjectsListConfigMap.Name,
							},
						},
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(credential, desiredPod, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
	}

	currentPod := &corev1.Pod{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredPod.Name,
			Namespace: desiredPod.Namespace,
		},
		currentPod,
	); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Create(ctx, desiredPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !equality.Semantic.DeepDerivative(desiredPod.Spec, currentPod.Spec) {
		if err := r.Update(ctx, desiredPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if currentPod.Status.Phase == corev1.PodSucceeded ||
		currentPod.Status.Phase == corev1.PodFailed {
		// As soon as the pod gets completed, delete it and its dependencies.
		if err := r.Delete(ctx, currentServiceAccount); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Delete(ctx, currentRole); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Delete(ctx, currentRoleBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if err := r.Delete(ctx, currentPod); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to bind: %w", err)
		}
		if currentPod.Status.Phase == corev1.PodFailed {
			// TODO: what should we do when the pod fails?
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// resourceForKind returns the APIResource object for a given Group and Kind. This is useful for
// obtaining the resource name as expected by the RBAC policy rules.
func (r *CredentialReconciler) resourceForKind(kwg kindWithGroup) (metav1.APIResource, error) {
	resource, exists := r.resourceCache[kwg]
	if !exists {
		// TODO: this is definitely not the best way of dealing with a missing resource in the local
		// cache. While this is fine for a prototype, it can crash the controller pod or the node
		// (depending on the pod resources) if the kind with the specified group doesn't exist on
		// the API server (or if the API server goes off for a little walk), or crash the node.
		if err := r.updateResourceCache(); err != nil {
			return metav1.APIResource{}, err
		}
		return r.resourceForKind(kwg)
	}
	return resource, nil
}

func (r *CredentialReconciler) updateResourceCache() error {
	lists, err := r.DiscoveryClient.ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("failed to update local resource cache: %w", err)
	}
	resources := make(map[kindWithGroup]metav1.APIResource)
	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			kwg := kindWithGroup{kind: resource.Kind, group: gv.Group}
			resources[kwg] = resource
		}
	}
	r.resourceCache = resources
	return nil
}

type kindWithGroup struct {
	kind, group string
}

func (kwg *kindWithGroup) String() string {
	return kwg.group + "/" + kwg.kind
}

// SetupWithManager configures the controller manager for the Credential resource.
func (r *CredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.DiscoveryClient = discoveryclient.NewDiscoveryClientForConfigOrDie(mgr.GetConfig())
	r.scheme = mgr.GetScheme()
	r.log = ctrl.Log.WithName("controllers").WithName("Credential")
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebrokerv1alpha1.Credential{}).
		Complete(r)
}
