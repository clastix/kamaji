// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kamajierrors "github.com/clastix/kamaji/internal/errors"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type Migrate struct {
	Client               client.Client
	KamajiNamespace      string
	KamajiServiceAccount string
	KamajiServiceName    string
	ShouldCleanUp        bool
	MigrateImage         string
	// WebhookCABundle is the CA bundle for the manager's admission webhook server, required to
	// install the tenant-side "kamaji-freeze" ValidatingWebhookConfiguration prior to starting
	// the migration Job: see ensureFreezeWebhook for why this must happen synchronously here.
	WebhookCABundle []byte

	actualDatastore  *kamajiv1alpha1.DataStore
	desiredDatastore *kamajiv1alpha1.DataStore
	job              *batchv1.Job

	inProgress bool
}

func (d *Migrate) GetHistogram() prometheus.Histogram {
	migrateCollector = resources.LazyLoadHistogramFromResource(migrateCollector, d)

	return migrateCollector
}

func (d *Migrate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if len(tenantControlPlane.Status.Storage.DataStoreName) == 0 {
		return nil
	}

	d.job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("migrate-%s", tenantControlPlane.UID),
			Namespace: d.KamajiNamespace,
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

	return d.Client.Get(ctx, types.NamespacedName{Name: tenantControlPlane.Spec.DataStore}, d.desiredDatastore)
}

func (d *Migrate) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return d.ShouldCleanUp && *tcp.Status.Kubernetes.Version.Status == kamajiv1alpha1.VersionMigrating
}

func (d *Migrate) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	err := d.Client.Get(ctx, types.NamespacedName{Name: d.job.GetName(), Namespace: d.job.GetNamespace()}, d.job)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return false, d.Client.Delete(ctx, d.job)
}

func (d *Migrate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if d.desiredDatastore == nil {
		return controllerutil.OperationResultNone, nil
	}

	if d.actualDatastore.GetName() == d.desiredDatastore.GetName() {
		return controllerutil.OperationResultNone, nil
	}

	// The freeze webhook must be active in the tenant cluster before the migration Job is allowed
	// to start copying data: see ensureFreezeWebhook for why this has to be synchronous.
	if err := d.ensureFreezeWebhook(ctx, tenantControlPlane); err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("unable to install freeze webhook prior to migration: %w", err)
	}

	res, err := utilities.CreateOrUpdateWithConflict(ctx, d.Client, d.job, func() error {
		d.job.SetLabels(map[string]string{
			"tcp.kamaji.clastix.io/name":      tenantControlPlane.GetName(),
			"tcp.kamaji.clastix.io/namespace": tenantControlPlane.GetNamespace(),
			"kamaji.clastix.io/component":     "migrate",
		})

		d.job.Spec.Template.ObjectMeta.Labels = utilities.MergeMaps(d.job.Spec.Template.ObjectMeta.Labels, d.job.Spec.Template.ObjectMeta.Labels)
		d.job.Spec.Template.Spec.ServiceAccountName = d.KamajiServiceAccount
		d.job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
		// The Job only copies datastore data, so a fixed securityContext keeps it
		// admissible under the "restricted" Pod Security Standard.
		d.job.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		}
		if len(d.job.Spec.Template.Spec.Containers) == 0 {
			d.job.Spec.Template.Spec.Containers = append(d.job.Spec.Template.Spec.Containers, corev1.Container{})
		}
		d.job.Spec.Template.Spec.Containers[0].Name = "migrate"
		d.job.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		}
		d.job.Spec.Template.Spec.Containers[0].Image = d.MigrateImage
		d.job.Spec.Template.Spec.Containers[0].Args = []string{
			"migrate",
			fmt.Sprintf("--tenant-control-plane=%s/%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()),
			fmt.Sprintf("--target-datastore=%s", tenantControlPlane.Spec.DataStore),
		}

		if annotations := tenantControlPlane.GetAnnotations(); annotations != nil {
			v, _ := strconv.ParseBool(annotations["kamaji.clastix.io/cleanup-prior-migration"])
			d.job.Spec.Template.Spec.Containers[0].Args = append(d.job.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--cleanup-prior-migration=%t", v))

			if timeout, tErr := time.ParseDuration(annotations["kamaji.clastix.io/migration-timeout"]); tErr == nil {
				d.job.Spec.Template.Spec.Containers[0].Args = append(d.job.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--timeout=%s", timeout.String()))
			}
		}

		return nil
	})
	if err != nil {
		// Jobs are immutable, except for a tiny subset of fields:
		// these are useless for Kamaji, and we don't have proper RBAC.
		// If the Job has a UUID, it means it's an update, and we're expecting that error.
		if errors.IsForbidden(err) && d.job.UID != "" {
			_ = d.Client.Delete(ctx, d.job)

			return controllerutil.OperationResultNone, fmt.Errorf("migration job must be cretaed back due to immutable fields")
		}

		return res, fmt.Errorf("unable to launch migrate job: %w", err)
	}

	switch res {
	case controllerutil.OperationResultCreated, controllerutil.OperationResultUpdated:
		d.inProgress = true

		return resources.OperationResultEnqueueBack, nil
	case controllerutil.OperationResultNone:

		// Note: job.Status.Conditions can contain more than one condition on Kubernetes versions greater than v1.30
		for _, condition := range d.job.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return controllerutil.OperationResultNone, nil
			}
		}

		d.inProgress = true

		return controllerutil.OperationResultNone, kamajierrors.MigrationInProcessError{}
	default:
		return controllerutil.OperationResultNone, fmt.Errorf("unexpected status %s from the migration job", res)
	}
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

// ensureFreezeWebhook installs the tenant-side "kamaji-freeze" ValidatingWebhookConfiguration
// and waits for it to be persisted before the caller is allowed to start the migration Job.
//
// The soot Migrate controller also reacts to VersionMigrating and installs the same webhook,
// but it is level-triggered: it always reads the TenantControlPlane's *current* status rather
// than the status at the time its trigger fired. If the migration Job runs to completion (status
// flips Migrating -> Ready) faster than that controller's first reconcile after the transition,
// it observes Ready directly and never installs the webhook at all - the migration then runs
// with no write protection on the source datastore. Installing it here, synchronously and
// before the Job (and therefore before VersionMigrating is ever observable by any watcher),
// closes that race: by the time anything can see VersionMigrating, the webhook already exists.
func (d *Migrate) ensureFreezeWebhook(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantClient, err := utilities.GetTenantClient(ctx, d.Client, tenantControlPlane)
	if err != nil {
		return fmt.Errorf("unable to build tenant client: %w", err)
	}

	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: FreezeWebhookName,
		},
	}

	_, err = utilities.CreateOrUpdateWithConflict(ctx, tenantClient, webhook, func() error {
		webhook.Webhooks = BuildFreezeValidatingWebhookConfiguration(d.KamajiNamespace, d.KamajiServiceName, d.WebhookCABundle)

		return nil
	})

	return err
}
