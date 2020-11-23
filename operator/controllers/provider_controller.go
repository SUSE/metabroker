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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebrokerv1alpha1 "github.com/SUSE/metabroker/operator/api/v1alpha1"
)

// ProviderReconciler implements the Reconcile method for the Provider resource.
type ProviderReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=providers/status,verbs=get;update;patch

const providerReconcileTimeout = time.Second * 10

// Reconcile reconciles a Provider resource.
func (r *ProviderReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), providerReconcileTimeout)
	defer cancel()

	log := r.Log.WithValues("provider", req.NamespacedName)

	provider := &servicebrokerv1alpha1.Provider{}
	if err := r.Get(ctx, req.NamespacedName, provider); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Provider resource deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager configures the controller manager for the Provider resource.
func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebrokerv1alpha1.Provider{}).
		Complete(r)
}
