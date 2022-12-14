// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/utilities"
)

type Migrate struct {
	client client.Client
	logger logr.Logger

	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	WebhookNamespace          string
	WebhookServiceName        string
	WebhookCABundle           []byte
	TriggerChannel            chan event.GenericEvent
}

func (m *Migrate) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := m.GetTenantControlPlaneFunc()
	if err != nil {
		return reconcile.Result{}, err
	}
	// Cannot detect the status of the TenantControlPlane, enqueuing back
	if tcp.Status.Kubernetes.Version.Status == nil {
		return reconcile.Result{Requeue: true}, nil
	}

	switch *tcp.Status.Kubernetes.Version.Status {
	case v1alpha1.VersionMigrating:
		err = m.createOrUpdate(ctx)
	case v1alpha1.VersionReady:
		err = m.cleanup(ctx)
	}

	if err != nil {
		m.logger.Error(err, "reconciliation failed")

		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (m *Migrate) cleanup(ctx context.Context) error {
	if err := m.client.Delete(ctx, m.object()); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("unable to clean-up ValidationWebhook required for migration: %w", err)
	}

	return nil
}

func (m *Migrate) createOrUpdate(ctx context.Context) error {
	obj := m.object()

	_, err := utilities.CreateOrUpdateWithConflict(ctx, m.client, obj, func() error {
		obj.Webhooks = []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "migrate.kamaji.clastix.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      pointer.String(fmt.Sprintf("https://%s.%s.svc:443/migrate", m.WebhookServiceName, m.WebhookNamespace)),
					CABundle: m.WebhookCABundle,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{"*"},
							Scope: func(v admissionregistrationv1.ScopeType) *admissionregistrationv1.ScopeType {
								return &v
							}(admissionregistrationv1.AllScopes),
						},
					},
				},
				FailurePolicy: func(v admissionregistrationv1.FailurePolicyType) *admissionregistrationv1.FailurePolicyType {
					return &v
				}(admissionregistrationv1.Fail),
				MatchPolicy: func(v admissionregistrationv1.MatchPolicyType) *admissionregistrationv1.MatchPolicyType {
					return &v
				}(admissionregistrationv1.Equivalent),
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: metav1.LabelSelectorOpNotIn,
							Values: []string{
								"kube-system",
							},
						},
					},
				},
				SideEffects: func(v admissionregistrationv1.SideEffectClass) *admissionregistrationv1.SideEffectClass {
					return &v
				}(admissionregistrationv1.SideEffectClassNoneOnDryRun),
				TimeoutSeconds:          nil,
				AdmissionReviewVersions: []string{"v1"},
			},
		}

		return nil
	})

	return err
}

func (m *Migrate) SetupWithManager(mgr manager.Manager) error {
	m.client = mgr.GetClient()
	m.logger = mgr.GetLogger().WithName("migrate")
	m.TriggerChannel = make(chan event.GenericEvent)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&admissionregistrationv1.ValidatingWebhookConfiguration{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			vwc := m.object()

			return object.GetName() == vwc.GetName()
		}))).
		Watches(&source.Channel{Source: m.TriggerChannel}, &handler.EnqueueRequestForObject{}).
		Complete(m)
}

func (m *Migrate) object() *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kamaji-freeze",
		},
	}
}
