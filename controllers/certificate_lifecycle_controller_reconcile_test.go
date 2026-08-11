// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/metrics"
)

func TestCertificateLifecycleReconcileAppliesReconcileTimeout(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	capturing := &contextCapturingClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	const reconcileTimeout = 5 * time.Second

	s := &CertificateLifecycle{
		Deadline:         24 * time.Hour,
		ReconcileTimeout: reconcileTimeout,
		Metrics:          metrics.NewRecorder(prometheus.NewRegistry()),
		client:           capturing,
	}

	if _, err := s.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "missing"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContextDeadlineWithin(t, capturing.capturedDeadline, capturing.hasDeadline, reconcileTimeout)
}
