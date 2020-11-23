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

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebrokerv1alpha1 "github.com/SUSE/metabroker/operator/api/v1alpha1"
)

// OfferingReconciler implements the Reconcile method for the Offering resource.
type OfferingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=offerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=offerings/status,verbs=get;update;patch

const offeringReconcileTimeout = time.Second * 10

// Reconcile reconciles an Offering resource.
func (r *OfferingReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), offeringReconcileTimeout)
	defer cancel()

	log := r.Log.WithValues("offering", req.NamespacedName)

	offering := &servicebrokerv1alpha1.Offering{}
	if err := r.Get(ctx, req.NamespacedName, offering); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Offering resource deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	offeringNeedsUpdate := false

	if len(offering.OwnerReferences) == 0 {
		if err := r.setOwnership(ctx, &req, offering); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		offeringNeedsUpdate = true
	}

	if offering.Spec.ID == "" {
		id := uuid.Must(uuid.NewUUID()) // UUID v1
		offering.Spec.ID = id.String()
		offeringNeedsUpdate = true
	}

	if offeringNeedsUpdate {
		if err := r.Update(ctx, offering); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *OfferingReconciler) setOwnership(
	ctx context.Context,
	req *ctrl.Request,
	offering *servicebrokerv1alpha1.Offering,
) error {
	providerNamespacedName := req.NamespacedName
	providerNamespacedName.Name = offering.Spec.Provider
	provider := &servicebrokerv1alpha1.Provider{}
	if err := r.Get(ctx, providerNamespacedName, provider); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}
	if err := ctrl.SetControllerReference(provider, offering, r.Scheme); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}
	return nil
}

// SetupWithManager configures the controller manager for the Offering resource.
func (r *OfferingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebrokerv1alpha1.Offering{}).
		Complete(r)
}
