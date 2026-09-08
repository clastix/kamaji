// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestKubeconfigGeneratorWatcherReconcileAppliesReconcileTimeout(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	capturing := &contextCapturingClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	const reconcileTimeout = 3 * time.Second

	w := &KubeconfigGeneratorWatcher{
		Client:           capturing,
		GeneratorChan:    make(chan event.GenericEvent, 1),
		ReconcileTimeout: reconcileTimeout,
	}

	if _, err := w.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContextDeadlineWithin(t, capturing.capturedDeadline, capturing.hasDeadline, reconcileTimeout)
}
