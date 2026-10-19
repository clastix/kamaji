// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestTenantControlPlaneReconcilerReturnsDataStoreLookupError(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "tcp", UID: types.UID("datastore-lookup-error")},
		Spec:       kamajiv1alpha1.TenantControlPlaneSpec{DataStore: "missing"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tcp).Build()

	r := &TenantControlPlaneReconciler{
		Client:           cl,
		APIReader:        cl,
		ReconcileTimeout: time.Minute,
		clock:            clock.RealClock{},
	}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "tcp"}}); !k8serrors.IsNotFound(err) {
		t.Fatalf("expected the DataStore lookup error to be returned, got %v", err)
	}
}
