// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type Migrate struct {
	Client               client.Client
	KamajiNamespace      string
	KamajiServiceAccount string
	KamajiServiceName    string
	CABundle             []byte
	ShouldCleanUp        bool

	actualDatastore  *kamajiv1alpha1.DataStore
	desiredDatastore *kamajiv1alpha1.DataStore
	job              *batchv1.Job
	webhook          *admissionregistrationv1.ValidatingWebhookConfiguration

	inProgress bool
}

func (d *Migrate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if len(tenantControlPlane.Status.Storage.DataStoreName) == 0 {
		return nil
	}

	d.job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("migrate-%s-%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()),
			Namespace: d.KamajiNamespace,
		},
	}

	d.webhook = &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kamaji-freeze",
			Namespace: "kube-system",
		},
	}

	if d.ShouldCleanUp {
		return nil
	}

	if err := d.Client.Get(ctx, types.NamespacedName{Name: d.job.GetName(), Namespace: d.job.GetNamespace()}, d.job); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	d.actualDatastore = &kamajiv1alpha1.DataStore{}
	if err := d.Client.Get(ctx, types.NamespacedName{Name: tenantControlPlane.Status.Storage.DataStoreName}, d.actualDatastore); err != nil {
		return err
	}

	d.desiredDatastore = &kamajiv1alpha1.DataStore{}
	if err := d.Client.Get(ctx, types.NamespacedName{Name: tenantControlPlane.Spec.DataStore}, d.desiredDatastore); err != nil {
		return err
	}

	return nil
}

func (d *Migrate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return d.ShouldCleanUp
}

func (d *Migrate) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	// Deleting migrate Job in the admin cluster
	var jobErr, webhookErr error

	if err := d.Client.Get(ctx, types.NamespacedName{Name: d.job.GetName(), Namespace: d.job.GetNamespace()}, d.job); err == nil {
		jobErr = d.Client.Delete(ctx, d.job)
	}
	// Deleting webhook deployed in the Tenant cluster
	tcpClient, err := utilities.GetTenantClient(ctx, d.Client, tcp)
	if err != nil {
		return false, fmt.Errorf("unable to create TenantControlPlane client: %w", err)
	}

	if err = tcpClient.Get(ctx, types.NamespacedName{Name: d.webhook.GetName(), Namespace: d.webhook.GetNamespace()}, d.webhook); err == nil {
		jobErr = tcpClient.Delete(ctx, d.webhook)
	}

	switch {
	case jobErr != nil:
		return false, jobErr
	case webhookErr != nil:
		return false, webhookErr
	default:
		return false, nil
	}
}

func (d *Migrate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if d.desiredDatastore == nil {
		return controllerutil.OperationResultNone, nil
	}

	if d.actualDatastore.GetName() == d.desiredDatastore.GetName() {
		return controllerutil.OperationResultNone, nil
	}

	tcpClient, err := utilities.GetTenantClient(ctx, d.Client, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("unable to create TenantControlPlane client: %w", err)
	}

	jobRessult, err := utilities.CreateOrUpdateWithConflict(ctx, d.Client, d.job, func() error {
		d.job.SetLabels(map[string]string{
			"tcp.kamaji.clastix.io/name":      tenantControlPlane.GetName(),
			"tcp.kamaji.clastix.io/namespace": tenantControlPlane.GetNamespace(),
			"kamaji.clastix.io/component":     "migrate",
		})

		d.job.Spec.Template.ObjectMeta.Labels = utilities.MergeMaps(d.job.Spec.Template.ObjectMeta.Labels, d.job.Spec.Template.ObjectMeta.Labels)
		d.job.Spec.Template.Spec.ServiceAccountName = d.KamajiServiceAccount
		d.job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
		if len(d.job.Spec.Template.Spec.Containers) == 0 {
			d.job.Spec.Template.Spec.Containers = append(d.job.Spec.Template.Spec.Containers, corev1.Container{})
		}
		d.job.Spec.Template.Spec.Containers[0].Name = "migrate"
		d.job.Spec.Template.Spec.Containers[0].Image = "clastix/kamaji:v0.1.1"
		d.job.Spec.Template.Spec.Containers[0].Command = []string{"/kamaji"}
		d.job.Spec.Template.Spec.Containers[0].Args = []string{
			"migrate",
			fmt.Sprintf("--tenant-control-plane=%s/%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()),
			fmt.Sprintf("--target-datastore=%s", tenantControlPlane.Spec.DataStore),
		}

		return nil
	})
	if err != nil {
		return jobRessult, fmt.Errorf("unable to launch migrate job: %w", err)
	}

	webhookResult, err := utilities.CreateOrUpdateWithConflict(ctx, tcpClient, d.webhook, func() error {
		d.webhook.Webhooks = []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "migrate.kamaji.clastix.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      pointer.String(fmt.Sprintf("https://%s.%s.svc:443/migrate", d.KamajiServiceName, d.KamajiNamespace)),
					CABundle: d.CABundle,
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
	if err != nil {
		return webhookResult, fmt.Errorf("unable to create webhook on TenantControlPlane: %w", err)
	}

	switch jobRessult {
	case controllerutil.OperationResultNone:
		if len(d.job.Status.Conditions) == 0 {
			break
		}

		condition := d.job.Status.Conditions[0]
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return controllerutil.OperationResultNone, nil
		}

		log.FromContext(ctx).Info("migration job not yet completed", "reason", condition.Reason, "message", condition.Message)
	case controllerutil.OperationResultCreated:
		break
	default:
		return "", fmt.Errorf("unexpected status %s from the migration job", jobRessult)
	}

	d.inProgress = true

	return controllerutil.OperationResultNone, nil
}

func (d *Migrate) GetName() string {
	return "migrate"
}

func (d *Migrate) ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool {
	return d.inProgress
}

func (d *Migrate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if d.inProgress {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionMigrating
	}

	return nil
}
