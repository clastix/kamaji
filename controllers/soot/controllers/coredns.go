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

type CoreDNS struct {
	logger logr.Logger

	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
}

func (c *CoreDNS) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	tcp, err := c.GetTenantControlPlaneFunc()
	if err != nil {
		c.logger.Error(err, "cannot retrieve TenantControlPlane")

		return reconcile.Result{}, err
	}

	c.logger.Info("start processing")

	resource := &addons.CoreDNS{Client: c.AdminClient}

	result, handlingErr := resources.Handle(ctx, resource, tcp)
	if handlingErr != nil {
		c.logger.Error(handlingErr, "resource process failed", "resource", resource.GetName())

		return reconcile.Result{}, handlingErr
	}

	if result == controllerutil.OperationResultNone {
		c.logger.Info("reconciliation completed")

		return reconcile.Result{}, nil
	}

	if err = utils.UpdateStatus(ctx, c.AdminClient, tcp, resource); err != nil {
		c.logger.Error(err, "update status failed", "resource", resource.GetName())

		return reconcile.Result{}, err
	}

	c.logger.Info("reconciliation processed")

	return reconcile.Result{}, nil
}

func (c *CoreDNS) SetupWithManager(mgr manager.Manager) error {
	c.logger = mgr.GetLogger().WithName("coredns")
	c.TriggerChannel = make(chan event.GenericEvent)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&rbacv1.ClusterRoleBinding{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == kubeadm.CoreDNSClusterRoleBindingName
		}))).
		WatchesRawSource(&source.Channel{Source: c.TriggerChannel}, &handler.EnqueueRequestForObject{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Complete(c)
}
