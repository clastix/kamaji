// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

type KonnectivityAgent struct {
	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
}

func (k *KonnectivityAgent) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx, "controller", "konnectivity_agent")

	tcp, err := k.GetTenantControlPlaneFunc()
	if err != nil {
		logger.Error(err, "cannot retrieve TenantControlPlane")

		return reconcile.Result{}, err
	}

	for _, resource := range controllers.GetExternalKonnectivityResources(k.AdminClient) {
		logger.Info("start processing konnectivity resource", "resource", resource.GetName())

		result, handlingErr := resources.Handle(ctx, resource, tcp)
		if handlingErr != nil {
			logger.Error(handlingErr, "konnectivity resource process failed", "resource", resource.GetName())

			return reconcile.Result{}, handlingErr
		}

		if result == controllerutil.OperationResultNone {
			logger.Info("konnectivity resource reconciled", "resource", resource.GetName())

			continue
		}

		if err = utils.UpdateStatus(ctx, k.AdminClient, k.GetTenantControlPlaneFunc, resource); err != nil {
			logger.Error(err, "update of the resource failed", "resource", resource.GetName())

			return reconcile.Result{}, err
		}
	}

	logger.Info("reconciliation completed")

	return reconcile.Result{}, nil
}

func (k *KonnectivityAgent) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		WithLogger(mgr.GetLogger().WithName("konnectivity_agent")).
		For(&appsv1.DaemonSet{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == konnectivity.AgentName && object.GetNamespace() == konnectivity.AgentNamespace
		}))).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
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
		Watches(&source.Kind{Type: &v1.ClusterRoleBinding{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
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
		Watches(&source.Channel{Source: k.TriggerChannel}, &handler.EnqueueRequestForObject{}).
		Complete(k)
}
