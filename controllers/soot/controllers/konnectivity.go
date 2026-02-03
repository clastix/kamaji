// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"errors"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/clastix/kamaji/controllers"
	sooterrors "github.com/clastix/kamaji/controllers/soot/controllers/errors"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

type KonnectivityAgent struct {
	Logger                    logr.Logger
	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
	ControllerName            string
}

func (k *KonnectivityAgent) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := k.GetTenantControlPlaneFunc()
	if err != nil {
		if errors.Is(err, sooterrors.ErrPausedReconciliation) {
			k.Logger.Info(err.Error())

			return reconcile.Result{}, nil
		}

		k.Logger.Error(err, "cannot retrieve TenantControlPlane")

		return reconcile.Result{}, err
	}

	if tcp.Spec.Addons.Konnectivity == nil {
		return reconcile.Result{}, nil
	}

	for _, resource := range controllers.GetExternalKonnectivityResources(k.AdminClient) {
		k.Logger.Info("start processing", "resource", resource.GetName())

		result, handlingErr := resources.Handle(ctx, resource, tcp)
		if handlingErr != nil {
			k.Logger.Error(handlingErr, "resource process failed", "resource", resource.GetName())

			return reconcile.Result{}, handlingErr
		}

		if result == controllerutil.OperationResultNone {
			k.Logger.Info("resource processed", "resource", resource.GetName())

			continue
		}

		if err = utils.UpdateStatus(ctx, k.AdminClient, tcp, resource); err != nil {
			k.Logger.Error(err, "update status failed", "resource", resource.GetName())

			return reconcile.Result{}, err
		}
	}

	k.Logger.Info("reconciliation completed")

	return reconcile.Result{}, nil
}

func (k *KonnectivityAgent) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(k.ControllerName).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: ptr.To(true)}).
		For(&appsv1.DaemonSet{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == konnectivity.AgentName && object.GetNamespace() == konnectivity.AgentNamespace
		}))).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
			if object.GetName() == konnectivity.AgentName && object.GetNamespace() == konnectivity.AgentNamespace {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: object.GetNamespace(),
							Name:      object.GetName(),
						},
					},
				}
			}

			return nil
		})).
		Watches(&v1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
			if object.GetName() == konnectivity.CertCommonName {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name: konnectivity.CertCommonName,
						},
					},
				}
			}

			return nil
		})).
		WatchesRawSource(source.Channel(k.TriggerChannel, &handler.EnqueueRequestForObject{})).
		Complete(k)
}
