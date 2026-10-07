// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
	"github.com/clastix/kamaji/internal/datastore"
	"github.com/clastix/kamaji/internal/resources"
)

// emptyConnection reports that no user, database or grant exists, so Delete
// goes straight to the finalizer removal.
type emptyConnection struct {
	datastore.Connection
}

func (emptyConnection) UserExists(context.Context, string) (bool, error) { return false, nil }

func (emptyConnection) DBExists(context.Context, string) (bool, error) { return false, nil }

func (emptyConnection) GrantPrivilegesExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func newFailingUpdateSetup(t *testing.T, updateErr error) (*Setup, *kamajiv1alpha1.TenantControlPlane) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "tcp",
			Namespace:  "default",
			Finalizers: []string{finalizers.DatastoreFinalizer},
		},
	}
	tcp.Status.Storage.Config.SecretName = "tcp-datastore-config"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp-datastore-config", Namespace: "default"},
		Data: map[string][]byte{
			"DB_SCHEMA":   []byte("schema"),
			"DB_USER":     []byte("user"),
			"DB_PASSWORD": []byte("password"),
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tcp, secret).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
				return updateErr
			},
		}).
		Build()

	return &Setup{Client: c, Connection: emptyConnection{}}, tcp
}

func TestSetupDeleteReturnsFinalizerRemovalError(t *testing.T) {
	updateErr := errors.New("admission webhook denied the request")
	setup, tcp := newFailingUpdateSetup(t, updateErr)
	setup.resource = &SetupResource{schema: "schema", user: "user", password: "password"}

	if err := setup.Delete(context.Background(), tcp); !errors.Is(err, updateErr) {
		t.Fatalf("Delete() = %v, want the finalizer removal error %q so the reconciler requeues", err, updateErr)
	}
}

func TestHandleDeletionReturnsFinalizerRemovalError(t *testing.T) {
	updateErr := errors.New("admission webhook denied the request")
	setup, tcp := newFailingUpdateSetup(t, updateErr)

	if err := resources.HandleDeletion(context.Background(), setup, tcp); !errors.Is(err, updateErr) {
		t.Fatalf("HandleDeletion() = %v, want the finalizer removal error %q so the reconciler requeues", err, updateErr)
	}
}
