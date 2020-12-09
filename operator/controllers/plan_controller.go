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
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebrokerv1alpha1 "github.com/SUSE/metabroker/operator/api/v1alpha1"
)

// PlanReconciler implements the Reconcile method for the Plan resource.
type PlanReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=plans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicebroker.metabroker.suse.com,resources=plans/status,verbs=get;update;patch

const planReconcileTimeout = time.Second * 10

// Reconcile reconciles a Plan resource.
func (r *PlanReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), planReconcileTimeout)
	defer cancel()

	log := r.Log.WithValues("plan", req.NamespacedName)

	plan := &servicebrokerv1alpha1.Plan{}
	if err := r.Get(ctx, req.NamespacedName, plan); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Plan resource deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	planNeedsUpdate := false

	if len(plan.OwnerReferences) == 0 {
		if err := r.setControllingOwner(ctx, &req, plan); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			}
			return ctrl.Result{}, err
		}
		planNeedsUpdate = true
	}

	if plan.Spec.ID == "" {
		id := uuid.Must(uuid.NewUUID()) // UUID v1
		plan.Spec.ID = id.String()
		planNeedsUpdate = true
	}

	if planNeedsUpdate {
		if err := r.Update(ctx, plan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *PlanReconciler) setControllingOwner(
	ctx context.Context,
	req *ctrl.Request,
	plan *servicebrokerv1alpha1.Plan,
) error {
	offeringNamespacedName := req.NamespacedName
	offeringNamespacedName.Name = plan.Spec.Offering
	offering := &servicebrokerv1alpha1.Offering{}
	if err := r.Get(ctx, offeringNamespacedName, offering); err != nil {
		return err
	}
	if err := ctrl.SetControllerReference(offering, plan, r.Scheme); err != nil {
		return err
	}
	return nil
}

// SetupWithManager configures the controller manager for the Plan resource.
func (r *PlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebrokerv1alpha1.Plan{}).
		Complete(r)
}
