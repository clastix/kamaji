// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"errors"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kamajierrors "github.com/clastix/kamaji/internal/errors"
)

func newReplaceMigration(c client.Client, job *batchv1.Job) *Migrate {
	return &Migrate{
		Client:           c,
		actualDatastore:  &kamajiv1alpha1.DataStore{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		desiredDatastore: &kamajiv1alpha1.DataStore{ObjectMeta: metav1.ObjectMeta{Name: "dedicated"}},
		job:              job,
	}
}

func TestMigrateWaitsForDeletedJob(t *testing.T) {
	ctx := context.Background()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "migrate-tcp",
			Namespace:  "kamaji-system",
			Finalizers: []string{metav1.FinalizerDeleteDependents},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(job).Build()
	if err := c.Delete(ctx, job); err != nil {
		t.Fatalf("unable to delete Job: %v", err)
	}

	terminating := &batchv1.Job{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(job), terminating); err != nil {
		t.Fatalf("unable to get Job: %v", err)
	}

	d := newReplaceMigration(c, terminating)

	_, err := d.CreateOrUpdate(ctx, &kamajiv1alpha1.TenantControlPlane{})
	if !errors.As(err, &kamajierrors.MigrationInProcessError{}) {
		t.Fatalf("expected a migration in progress while the Job is deleted, got %v", err)
	}

	if !d.inProgress {
		t.Error("expected the migration to be reported in progress")
	}
}

func TestMigrateDeleteJobWaitsForPods(t *testing.T) {
	ctx := context.Background()

	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "migrate-tcp", Namespace: "kamaji-system"}}

	var propagation *metav1.DeletionPropagation

	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(job).WithInterceptorFuncs(interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			o := &client.DeleteOptions{}
			o.ApplyOptions(opts)
			propagation = o.PropagationPolicy

			return c.Delete(ctx, obj, opts...)
		},
	}).Build()

	if err := newReplaceMigration(c, job).deleteJob(ctx); err != nil {
		t.Fatalf("unable to delete Job: %v", err)
	}

	if propagation == nil || *propagation != metav1.DeletePropagationForeground {
		t.Errorf("expected foreground propagation, got %v", propagation)
	}
}
