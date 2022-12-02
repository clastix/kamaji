// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
	"github.com/clastix/kamaji/internal/datastore"
	kamajierrors "github.com/clastix/kamaji/internal/errors"
	"github.com/clastix/kamaji/internal/resources"
)

// TenantControlPlaneReconciler reconciles a TenantControlPlane object.
type TenantControlPlaneReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Config      TenantControlPlaneReconcilerConfig
	TriggerChan TenantControlPlaneChannel
}

// TenantControlPlaneReconcilerConfig gives the necessary configuration for TenantControlPlaneReconciler.
type TenantControlPlaneReconcilerConfig struct {
	DefaultDataStoreName string
	KineContainerImage   string
	TmpBaseDirectory     string
}

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *TenantControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	tenantControlPlane := &kamajiv1alpha1.TenantControlPlane{}
	isTenantControlPlane, err := r.getTenantControlPlane(ctx, req.NamespacedName, tenantControlPlane)
	if err != nil {
		log.Error(err, "cannot retrieve the required instance")

		return ctrl.Result{}, err
	}
	if !isTenantControlPlane {
		return ctrl.Result{}, nil
	}

	markedToBeDeleted := tenantControlPlane.GetDeletionTimestamp() != nil

	if markedToBeDeleted && len(tenantControlPlane.Finalizers) == 0 {
		return ctrl.Result{}, nil
	}
	// Retrieving the DataStore to use for the current reconciliation
	ds, err := r.dataStore(ctx, tenantControlPlane)
	if err != nil {
		log.Error(err, "cannot retrieve the DataStore for the given instance")

		return ctrl.Result{}, err
	}

	dsConnection, err := datastore.NewStorageConnection(ctx, r.Client, *ds)
	if err != nil {
		log.Error(err, "cannot generate the DataStore connection for the given instance")

		return ctrl.Result{}, err
	}
	defer dsConnection.Close()

	if markedToBeDeleted {
		log.Info("marked for deletion, performing clean-up")

		groupDeletableResourceBuilderConfiguration := GroupDeletableResourceBuilderConfiguration{
			client:              r.Client,
			log:                 log,
			tcpReconcilerConfig: r.Config,
			tenantControlPlane:  *tenantControlPlane,
			connection:          dsConnection,
		}

		for _, resource := range GetDeletableResources(tenantControlPlane, groupDeletableResourceBuilderConfiguration) {
			if err = resources.HandleDeletion(ctx, resource, tenantControlPlane); err != nil {
				log.Error(err, "resource deletion failed", "resource", resource.GetName())

				return ctrl.Result{}, err
			}
		}

		log.Info("resource deletions have been completed")

		return ctrl.Result{}, nil
	}

	groupResourceBuilderConfiguration := GroupResourceBuilderConfiguration{
		client:              r.Client,
		log:                 log,
		tcpReconcilerConfig: r.Config,
		tenantControlPlane:  *tenantControlPlane,
		DataStore:           *ds,
		Connection:          dsConnection,
	}
	registeredResources := GetResources(groupResourceBuilderConfiguration)

	for _, resource := range registeredResources {
		result, err := resources.Handle(ctx, resource, tenantControlPlane)
		if err != nil {
			if kamajierrors.ShouldReconcileErrorBeIgnored(err) {
				log.V(1).Info("sentinel error, enqueuing back request", "error", err.Error())

				return ctrl.Result{Requeue: true}, nil
			}

			log.Error(err, "handling of resource failed", "resource", resource.GetName())

			return ctrl.Result{}, err
		}

		if result == controllerutil.OperationResultNone {
			continue
		}

		if err := r.updateStatus(ctx, req.NamespacedName, resource); err != nil {
			log.Error(err, "update of the resource failed", "resource", resource.GetName())

			return ctrl.Result{}, err
		}

		log.Info(fmt.Sprintf("%s has been configured", resource.GetName()))

		return ctrl.Result{}, nil
	}

	log.Info(fmt.Sprintf("%s has been reconciled", tenantControlPlane.GetName()))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Channel{Source: r.TriggerChan}, handler.Funcs{GenericFunc: func(genericEvent event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {
			limitingInterface.AddRateLimited(ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{
					Namespace: genericEvent.Object.GetNamespace(),
					Name:      genericEvent.Object.GetName(),
				},
			})
		}}).
		For(&kamajiv1alpha1.TenantControlPlane{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func (r *TenantControlPlaneReconciler) getTenantControlPlane(ctx context.Context, namespacedName k8stypes.NamespacedName, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Get(ctx, namespacedName, tenantControlPlane); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *TenantControlPlaneReconciler) updateStatus(ctx context.Context, namespacedName k8stypes.NamespacedName, resource resources.Resource) error {
	tenantControlPlane := &kamajiv1alpha1.TenantControlPlane{}

	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		isTenantControlPlane, err := r.getTenantControlPlane(ctx, namespacedName, tenantControlPlane)
		if err != nil {
			return err
		}

		if !isTenantControlPlane {
			return fmt.Errorf("error updating tenantControlPlane %s: not found", namespacedName.Name)
		}

		if err = resource.UpdateTenantControlPlaneStatus(ctx, tenantControlPlane); err != nil {
			return fmt.Errorf("error applying TenantcontrolPlane status: %w", err)
		}

		if err = r.Status().Update(ctx, tenantControlPlane); err != nil {
			return fmt.Errorf("error updating tenantControlPlane status: %w", err)
		}

		return nil
	})

	return updateErr
}

func (r *TenantControlPlaneReconciler) RemoveFinalizer(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	controllerutil.RemoveFinalizer(tenantControlPlane, finalizers.TenantControlPlaneFinalizer)

	return r.Update(ctx, tenantControlPlane)
}

// dataStore retrieves the override DataStore for the given Tenant Control Plane if specified,
// otherwise fallback to the default one specified in the Kamaji setup.
func (r *TenantControlPlaneReconciler) dataStore(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kamajiv1alpha1.DataStore, error) {
	dataStoreName := tenantControlPlane.Spec.DataStore
	if len(dataStoreName) == 0 {
		dataStoreName = r.Config.DefaultDataStoreName
	}

	if statusDataStore := tenantControlPlane.Status.Storage.DataStoreName; len(statusDataStore) > 0 && dataStoreName != statusDataStore {
		return nil, fmt.Errorf("migration from a DataStore (current: %s) to another one (desired: %s) is not yet supported", statusDataStore, dataStoreName)
	}

	ds := &kamajiv1alpha1.DataStore{}
	if err := r.Client.Get(ctx, k8stypes.NamespacedName{Name: dataStoreName}, ds); err != nil {
		return nil, errors.Wrap(err, "cannot retrieve *kamajiv1alpha.DataStore object")
	}

	return ds, nil
}
