// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"github.com/go-logr/logr"
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
	logger logr.Logger

	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
}

func (k *KonnectivityAgent) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := k.GetTenantControlPlaneFunc()
	if err != nil {
		k.logger.Error(err, "cannot retrieve TenantControlPlane")

		return reconcile.Result{}, err
	}

	for _, resource := range controllers.GetExternalKonnectivityResources(k.AdminClient) {
		k.logger.Info("start processing", "resource", resource.GetName())

		result, handlingErr := resources.Handle(ctx, resource, tcp)
		if handlingErr != nil {
			k.logger.Error(handlingErr, "resource process failed", "resource", resource.GetName())

			return reconcile.Result{}, handlingErr
		}

		if result == controllerutil.OperationResultNone {
			k.logger.Info("resource processed", "resource", resource.GetName())

			continue
		}

		if err = utils.UpdateStatus(ctx, k.AdminClient, k.GetTenantControlPlaneFunc, resource); err != nil {
			k.logger.Error(err, "update status failed", "resource", resource.GetName())

			return reconcile.Result{}, err
		}
	}

	k.logger.Info("reconciliation completed")

	return reconcile.Result{}, nil
}

func (k *KonnectivityAgent) SetupWithManager(mgr manager.Manager) error {
	k.logger = mgr.GetLogger().WithName("konnectivity_agent")
	k.TriggerChannel = make(chan event.GenericEvent)

	return controllerruntime.NewControllerManagedBy(mgr).
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
