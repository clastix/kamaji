// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kamajierrors "github.com/clastix/kamaji/internal/errors"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/sql"
	"github.com/clastix/kamaji/internal/types"
)

const (
	finalizer = "finalizer.kamaji.clastix.io"
)

// TenantControlPlaneReconciler reconciles a TenantControlPlane object.
type TenantControlPlaneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config TenantControlPlaneReconcilerConfig
}

// TenantControlPlaneReconcilerConfig gives the necessary configuration for TenantControlPlaneReconciler.
type TenantControlPlaneReconcilerConfig struct {
	ETCDStorageType           types.ETCDStorageType
	ETCDCASecretName          string
	ETCDCASecretNamespace     string
	ETCDClientSecretName      string
	ETCDClientSecretNamespace string
	ETCDEndpoints             string
	ETCDCompactionInterval    string
	TmpBaseDirectory          string
	DBConnection              sql.DBConnection
	KineMySQLSecretName       string
	KineMySQLSecretNamespace  string
	KineMySQLHost             string
	KineMySQLPort             int
}

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *TenantControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	tenantControlPlane := &kamajiv1alpha1.TenantControlPlane{}
	isTenantControlPlane, err := r.getTenantControlPlane(ctx, req.NamespacedName, tenantControlPlane)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !isTenantControlPlane {
		return ctrl.Result{}, nil
	}

	markedToBeDeleted := tenantControlPlane.GetDeletionTimestamp() != nil
	hasFinalizer := hasFinalizer(*tenantControlPlane)

	if markedToBeDeleted && !hasFinalizer {
		return ctrl.Result{}, nil
	}

	dbConnection, err := r.getStorageConnection(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		// TODO: Currently, etcd is not accessed using this dbConnection. For that reason we need this check
		// Check: https://github.com/clastix/kamaji/issues/67
		if dbConnection != nil {
			dbConnection.Close()
		}
	}()

	if markedToBeDeleted {
		log.Info("marked for deletion, performing clean-up")

		groupDeleteableResourceBuilderConfiguration := GroupDeleteableResourceBuilderConfiguration{
			client:              r.Client,
			log:                 log,
			tcpReconcilerConfig: r.Config,
			tenantControlPlane:  *tenantControlPlane,
			DBConnection:        dbConnection,
		}
		registeredDeletableResources := GetDeletableResources(groupDeleteableResourceBuilderConfiguration)

		for _, resource := range registeredDeletableResources {
			if err := resources.HandleDeletion(ctx, resource, tenantControlPlane); err != nil {
				return ctrl.Result{}, err
			}
		}

		if hasFinalizer {
			log.Info("removing finalizer")

			if err := r.RemoveFinalizer(ctx, tenantControlPlane); err != nil {
				return ctrl.Result{}, err
			}
		}

		log.Info("resource deletion has been completed")

		return ctrl.Result{}, nil
	}

	if !hasFinalizer {
		return ctrl.Result{}, r.AddFinalizer(ctx, tenantControlPlane)
	}

	groupResourceBuilderConfiguration := GroupResourceBuilderConfiguration{
		client:              r.Client,
		log:                 log,
		tcpReconcilerConfig: r.Config,
		tenantControlPlane:  *tenantControlPlane,
		DBConnection:        dbConnection,
	}
	registeredResources := GetResources(groupResourceBuilderConfiguration)

	for _, resource := range registeredResources {
		result, err := resources.Handle(ctx, resource, tenantControlPlane)
		if err != nil {
			if kamajierrors.ShouldReconcileErrorBeIgnored(err) {
				log.V(1).Info("sentinel error, enqueuing back request", "error", err.Error())

				return ctrl.Result{Requeue: true}, nil
			}

			return ctrl.Result{}, err
		}

		if result == controllerutil.OperationResultNone {
			continue
		}

		if err := r.updateStatus(ctx, req.NamespacedName, resource); err != nil {
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
	isTenantControlPlane, err := r.getTenantControlPlane(ctx, namespacedName, tenantControlPlane)
	if err != nil {
		return err
	}

	if !isTenantControlPlane {
		return fmt.Errorf("error updating tenantControlPlane %s: not found", namespacedName.Name)
	}

	if err := resource.UpdateTenantControlPlaneStatus(ctx, tenantControlPlane); err != nil {
		return err
	}

	if err := r.Status().Update(ctx, tenantControlPlane); err != nil {
		return fmt.Errorf("error updating tenantControlPlane status: %w", err)
	}

	return nil
}

func hasFinalizer(tenantControlPlane kamajiv1alpha1.TenantControlPlane) bool {
	for _, f := range tenantControlPlane.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}

	return false
}

func (r *TenantControlPlaneReconciler) AddFinalizer(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	controllerutil.AddFinalizer(tenantControlPlane, finalizer)

	return r.Update(ctx, tenantControlPlane)
}

func (r *TenantControlPlaneReconciler) RemoveFinalizer(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	controllerutil.RemoveFinalizer(tenantControlPlane, finalizer)

	return r.Update(ctx, tenantControlPlane)
}
