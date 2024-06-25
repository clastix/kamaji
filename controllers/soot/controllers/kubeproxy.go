// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/addons"
)

type KubeProxy struct {
	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent

	logger logr.Logger
}

func (k *KubeProxy) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := k.GetTenantControlPlaneFunc()
	if err != nil {
		k.logger.Error(err, "cannot retrieve TenantControlPlane")

		return reconcile.Result{}, err
	}

	k.logger.Info("start processing")

	resource := &addons.KubeProxy{Client: k.AdminClient}

	result, handlingErr := resources.Handle(ctx, resource, tcp)
	if handlingErr != nil {
		k.logger.Error(handlingErr, "resource process failed", "resource", resource.GetName())

		return reconcile.Result{}, handlingErr
	}

	if result == controllerutil.OperationResultNone {
		k.logger.Info("reconciliation completed")

		return reconcile.Result{}, nil
	}

	if err = utils.UpdateStatus(ctx, k.AdminClient, tcp, resource); err != nil {
		k.logger.Error(err, "update status failed")

		return reconcile.Result{}, err
	}

	k.logger.Info("reconciliation processed")

	return reconcile.Result{}, nil
}

func (k *KubeProxy) SetupWithManager(mgr manager.Manager) error {
	k.logger = mgr.GetLogger().WithName("kube_proxy")
	k.TriggerChannel = make(chan event.GenericEvent)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&rbacv1.ClusterRoleBinding{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == kubeadm.KubeProxyClusterRoleBindingName
		}))).
		WatchesRawSource(source.Channel(k.TriggerChannel, &handler.EnqueueRequestForObject{})).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.DaemonSet{}).
		Complete(k)
}
