// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	ds "github.com/clastix/kamaji/internal/resources/datastore"
)

func TestMigrateReconcileFreezeWebhook(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		status      kamajiv1alpha1.KubernetesVersionStatus
		existing    bool
		wantPresent bool
	}{
		{name: "migrating installs the webhook", status: kamajiv1alpha1.VersionMigrating, wantPresent: true},
		{name: "ready removes the webhook", status: kamajiv1alpha1.VersionReady, existing: true},
		{name: "write limited removes the webhook", status: kamajiv1alpha1.VersionWriteLimited, existing: true},
		{name: "not ready keeps the webhook", status: kamajiv1alpha1.VersionNotReady, existing: true, wantPresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			if err := admissionregistrationv1.AddToScheme(scheme); err != nil {
				t.Fatalf("failed adding admissionregistration scheme: %v", err)
			}

			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.existing {
				builder = builder.WithObjects(&admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: ds.FreezeWebhookName},
					Webhooks:   ds.BuildFreezeValidatingWebhookConfiguration("kamaji-system", "kamaji-webhook-service", []byte("ca")),
				})
			}
			cl := builder.Build()

			tcp := &kamajiv1alpha1.TenantControlPlane{}
			tcp.Status.Kubernetes.Version.Status = &tc.status

			m := &Migrate{
				Client:                    cl,
				Logger:                    logr.Discard(),
				GetTenantControlPlaneFunc: func() (*kamajiv1alpha1.TenantControlPlane, error) { return tcp, nil },
				WebhookNamespace:          "kamaji-system",
				WebhookServiceName:        "kamaji-webhook-service",
				WebhookCABundle:           []byte("ca"),
			}

			if _, err := m.Reconcile(t.Context(), reconcile.Request{}); err != nil {
				t.Fatalf("unexpected reconcile error: %v", err)
			}

			err := cl.Get(t.Context(), types.NamespacedName{Name: ds.FreezeWebhookName}, &admissionregistrationv1.ValidatingWebhookConfiguration{})
			switch {
			case tc.wantPresent && err != nil:
				t.Fatalf("expected the %s webhook to exist, got %v", ds.FreezeWebhookName, err)
			case !tc.wantPresent && !apierrors.IsNotFound(err):
				t.Fatalf("expected the %s webhook to be removed, got %v", ds.FreezeWebhookName, err)
			}
		})
	}
}
