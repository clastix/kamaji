// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type DatastoreMigrate struct {
	Client               client.Client
	KamajiNamespace      string
	KamajiServiceAccount string
	MigrateImage         string
	ShouldCleanUp        bool

	actualDatastore  *kamajiv1alpha1.DataStore
	desiredDatastore *kamajiv1alpha1.DataStore
	job              *batchv1.Job

	inProgress bool
}

func (d *DatastoreMigrate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if len(tenantControlPlane.Status.Storage.DataStoreName) == 0 {
		return nil
	}

	d.job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("migrate-%s-%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()),
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
	if err := d.Client.Get(ctx, types.NamespacedName{Name: tenantControlPlane.Spec.DataStore}, d.desiredDatastore); err != nil {
		return err
	}

	return nil
}

func (d *DatastoreMigrate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return d.ShouldCleanUp
}

func (d *DatastoreMigrate) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := d.Client.Get(ctx, types.NamespacedName{Name: d.job.GetName(), Namespace: d.job.GetNamespace()}, d.job); err != nil && errors.IsNotFound(err) {
		return false, nil
	}

	err := d.Client.Delete(ctx, d.job)

	return err == nil, err
}

func (d *DatastoreMigrate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if d.desiredDatastore == nil {
		return controllerutil.OperationResultNone, nil
	}

	if d.actualDatastore.GetName() == d.desiredDatastore.GetName() {
		return controllerutil.OperationResultNone, nil
	}

	res, err := utilities.CreateOrUpdateWithConflict(ctx, d.Client, d.job, func() error {
		d.job.SetLabels(map[string]string{
			"tcp.kamaji.clastix.io/name":       tenantControlPlane.GetName(),
			"tcp.kamaji.clastix.io/namespace":  tenantControlPlane.GetNamespace(),
			"kamaji.clastix.io/component":      "migrate",
			"migrate.kamaji.clastix.io/driver": tenantControlPlane.Status.Storage.Driver,
			"migrate.kamaji.clastix.io/from":   tenantControlPlane.Status.Storage.DataStoreName,
			"migrate.kamaji.clastix.io/to":     tenantControlPlane.Spec.DataStore,
		})

		d.job.Spec.Template.ObjectMeta.Labels = utilities.MergeMaps(d.job.Spec.Template.ObjectMeta.Labels, d.job.Spec.Template.ObjectMeta.Labels)
		d.job.Spec.Template.Spec.ServiceAccountName = d.KamajiServiceAccount
		d.job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
		if len(d.job.Spec.Template.Spec.Containers) == 0 {
			d.job.Spec.Template.Spec.Containers = append(d.job.Spec.Template.Spec.Containers, corev1.Container{})
		}
		d.job.Spec.Template.Spec.Containers[0].Name = "migrate"
		d.job.Spec.Template.Spec.Containers[0].Image = d.MigrateImage
		d.job.Spec.Template.Spec.Containers[0].Command = []string{"/kamaji"}
		d.job.Spec.Template.Spec.Containers[0].Args = []string{
			"migrate",
			fmt.Sprintf("--tenant-control-plane=%s/%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()),
			fmt.Sprintf("--target-datastore=%s", tenantControlPlane.Spec.DataStore),
		}
		// Allowing custom migrate timeout by reading TCP annotation
		annotations := tenantControlPlane.GetAnnotations()
		if v, ok := annotations["migrate.kamaji.clastix.io/timeout"]; ok {
			timeout, parseErr := time.ParseDuration(v)
			if parseErr != nil {
				log.FromContext(ctx).Error(parseErr, "cannot override default migrate timeout")
			} else {
				d.job.Spec.Template.Spec.Containers[0].Args = append(d.job.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--timeout=%s", timeout))
			}
		}

		return nil
	})
	if err != nil {
		return res, err
	}

	switch res {
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
		return "", fmt.Errorf("unexpected status %s from the migration job", res)
	}

	d.inProgress = true

	return controllerutil.OperationResultNone, nil
}

func (d *DatastoreMigrate) GetName() string {
	return "migrate"
}

func (d *DatastoreMigrate) ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool {
	return d.inProgress
}

func (d *DatastoreMigrate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if d.inProgress {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionMigrating
	}

	return nil
}
