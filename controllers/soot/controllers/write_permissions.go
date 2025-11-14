// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/errors"
	"k8s.io/utils/ptr"
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

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	sooterrors "github.com/clastix/kamaji/controllers/soot/controllers/errors"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/utilities"
)

type WritePermissions struct {
	Logger                    logr.Logger
	Client                    client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	WebhookNamespace          string
	WebhookServiceName        string
	WebhookCABundle           []byte
	TriggerChannel            chan event.GenericEvent
}

func (r *WritePermissions) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := r.GetTenantControlPlaneFunc()
	if err != nil {
		if errors.Is(err, sooterrors.ErrPausedReconciliation) {
			r.Logger.Info(err.Error())

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}
	// Cannot detect the status of the TenantControlPlane, enqueuing back
	if tcp.Status.Kubernetes.Version.Status == nil {
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	switch {
	case ptr.Deref(tcp.Status.Kubernetes.Version.Status, kamajiv1alpha1.VersionUnknown) == kamajiv1alpha1.VersionWriteLimited &&
		tcp.Spec.WritePermissions.HasAnyLimitation():
		err = r.createOrUpdate(ctx, tcp.Spec.WritePermissions)
	default:
		err = r.cleanup(ctx)
	}

	if err != nil {
		r.Logger.Error(err, "reconciliation failed")

		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *WritePermissions) createOrUpdate(ctx context.Context, writePermissions kamajiv1alpha1.Permissions) error {
	obj := r.object().DeepCopy()

	_, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, obj, func() error {
		obj.Webhooks = []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "leases.write-permissions.kamaji.clastix.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To(fmt.Sprintf("https://%s.%s.svc:443/write-permission", r.WebhookServiceName, r.WebhookNamespace)),
					CABundle: r.WebhookCABundle,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Delete,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{"*"},
							Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
						},
					},
				},
				FailurePolicy: ptr.To(admissionregistrationv1.Fail),
				MatchPolicy:   ptr.To(admissionregistrationv1.Equivalent),
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: metav1.LabelSelectorOpIn,
							Values: []string{
								"kube-node-lease",
							},
						},
					},
				},
				SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNoneOnDryRun),
				AdmissionReviewVersions: []string{"v1"},
			},
			{
				Name: "catchall.write-permissions.kamaji.clastix.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To(fmt.Sprintf("https://%s.%s.svc:443/write-permission", r.WebhookServiceName, r.WebhookNamespace)),
					CABundle: r.WebhookCABundle,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: func() []admissionregistrationv1.OperationType {
							var ops []admissionregistrationv1.OperationType

							if writePermissions.BlockCreate {
								ops = append(ops, admissionregistrationv1.Create)
							}

							if writePermissions.BlockUpdate {
								ops = append(ops, admissionregistrationv1.Update)
							}

							if writePermissions.BlockDelete {
								ops = append(ops, admissionregistrationv1.Delete)
							}

							return ops
						}(),
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{"*"},
							Scope:       ptr.To(admissionregistrationv1.AllScopes),
						},
					},
				},
				FailurePolicy: ptr.To(admissionregistrationv1.Fail),
				MatchPolicy:   ptr.To(admissionregistrationv1.Equivalent),
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: metav1.LabelSelectorOpNotIn,
							Values: []string{
								"kube-system",
								"kube-node-lease",
							},
						},
					},
				},
				SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNoneOnDryRun),
				TimeoutSeconds:          nil,
				AdmissionReviewVersions: []string{"v1"},
			},
		}

		return nil
	})

	return err
}

func (r *WritePermissions) cleanup(ctx context.Context) error {
	if err := r.Client.Delete(ctx, r.object()); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("unable to clean-up ValidationWebhook required for write permissions: %w", err)
	}

	return nil
}

func (r *WritePermissions) SetupWithManager(mgr manager.Manager) error {
	r.TriggerChannel = make(chan event.GenericEvent)

	return controllerruntime.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: ptr.To(true)}).
		For(&admissionregistrationv1.ValidatingWebhookConfiguration{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == r.object().GetName()
		}))).
		WatchesRawSource(source.Channel(r.TriggerChannel, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

func (r *WritePermissions) object() *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kamaji-write-permissions",
		},
	}
}
