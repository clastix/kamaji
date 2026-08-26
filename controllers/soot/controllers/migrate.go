// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/clastix/kamaji/api/v1alpha1"
	sooterrors "github.com/clastix/kamaji/controllers/soot/controllers/errors"
	"github.com/clastix/kamaji/controllers/utils"
	ds "github.com/clastix/kamaji/internal/resources/datastore"
	"github.com/clastix/kamaji/internal/utilities"
)

type Migrate struct {
	Client                    client.Client
	Logger                    logr.Logger
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	WebhookNamespace          string
	WebhookServiceName        string
	WebhookCABundle           []byte
	TriggerChannel            chan event.GenericEvent
	ControllerName            string
}

func (m *Migrate) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := m.GetTenantControlPlaneFunc()
	if err != nil {
		if errors.Is(err, sooterrors.ErrPausedReconciliation) {
			m.Logger.Info(err.Error())

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}
	// Cannot detect the status of the TenantControlPlane, enqueuing back
	if tcp.Status.Kubernetes.Version.Status == nil {
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	switch *tcp.Status.Kubernetes.Version.Status {
	case v1alpha1.VersionMigrating:
		err = m.createOrUpdate(ctx)
	case v1alpha1.VersionReady:
		err = m.cleanup(ctx)
	}

	if err != nil {
		m.Logger.Error(err, "reconciliation failed")

		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (m *Migrate) cleanup(ctx context.Context) error {
	if err := m.Client.Delete(ctx, m.object()); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("unable to clean-up ValidationWebhook required for migration: %w", err)
	}

	return nil
}

func (m *Migrate) createOrUpdate(ctx context.Context) error {
	obj := m.object()

	_, err := utilities.CreateOrUpdateWithConflict(ctx, m.Client, obj, func() error {
		obj.Webhooks = ds.BuildFreezeValidatingWebhookConfiguration(m.WebhookNamespace, m.WebhookServiceName, m.WebhookCABundle)

		return nil
	})

	return err
}

func (m *Migrate) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(m.ControllerName).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: pointer.To(true)}).
		For(&admissionregistrationv1.ValidatingWebhookConfiguration{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			vwc := m.object()

			return object.GetName() == vwc.GetName()
		}))).
		WatchesRawSource(source.Channel(m.TriggerChannel, &handler.EnqueueRequestForObject{})).
		Complete(m)
}

func (m *Migrate) object() *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: ds.FreezeWebhookName,
		},
	}
}
