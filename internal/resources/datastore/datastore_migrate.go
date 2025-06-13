// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	res, err := utilities.CreateOrUpdateWithConflict(ctx, d.Client, d.job, func() error {
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
		d.job.Spec.Template.Spec.Containers[0].Image = d.MigrateImage
		d.job.Spec.Template.Spec.Containers[0].Args = []string{
			"migrate",
			fmt.Sprintf("--tenant-control-plane=%s/%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()),
			fmt.Sprintf("--target-datastore=%s", tenantControlPlane.Spec.DataStore),
		}

		if tenantControlPlane.GetAnnotations() != nil {
			v, _ := strconv.ParseBool(tenantControlPlane.GetAnnotations()["kamaji.clastix.io/cleanup-prior-migration"])
			d.job.Spec.Template.Spec.Containers[0].Args = append(d.job.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--cleanup-prior-migration=%t", v))
		}

		return nil
	})
	if err != nil {
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
