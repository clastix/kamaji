// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/resources"
)

type KubeadmPhase struct {
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
	Phase                     resources.KubeadmPhaseResource

	logger logr.Logger
}

func (k *KubeadmPhase) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := k.GetTenantControlPlaneFunc()
	if err != nil {
		return reconcile.Result{}, err
	}

	k.logger.Info("start processing")

	result, handlingErr := resources.Handle(ctx, k.Phase, tcp)
	if handlingErr != nil {
		k.logger.Error(handlingErr, "resource process failed")

		return reconcile.Result{}, handlingErr
	}

	if result == controllerutil.OperationResultNone {
		k.logger.Info("reconciliation completed")

		return reconcile.Result{}, nil
	}

	if err = utils.UpdateStatus(ctx, k.Phase.GetClient(), k.GetTenantControlPlaneFunc, k.Phase); err != nil {
		k.logger.Error(err, "update status failed")

		return reconcile.Result{}, err
	}

	k.logger.Info("reconciliation processed")

	return reconcile.Result{}, nil
}

func (k *KubeadmPhase) SetupWithManager(mgr manager.Manager) error {
	k.logger = mgr.GetLogger().WithName(k.Phase.GetName())

	return controllerruntime.NewControllerManagedBy(mgr).
		For(k.Phase.GetWatchedObject(), builder.WithPredicates(predicate.NewPredicateFuncs(k.Phase.GetPredicateFunc()))).
		Watches(&source.Channel{Source: k.TriggerChannel}, &handler.EnqueueRequestForObject{}).
		Complete(k)
}
