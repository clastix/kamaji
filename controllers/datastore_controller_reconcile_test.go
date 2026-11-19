// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/metrics"
)

func TestDataStoreReconcileValidConditionObservesCurrentGeneration(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	ds := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "primary",
			Generation: 2,
			Finalizers: []string{kamajiv1alpha1.DataStoreTCPFinalizer},
		},
		Spec: kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.KinePostgreSQLDriver},
		Status: kamajiv1alpha1.DataStoreStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{{
				Type:               kamajiv1alpha1.DataStoreConditionValidType,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 1,
				Reason:             "MissingCondition",
			}},
		},
	}

	indexer := &kamajiv1alpha1.TenantControlPlaneStatusDataStore{}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ds).
		WithStatusSubresource(ds).
		WithIndex(indexer.Object(), indexer.Field(), indexer.ExtractValue()).
		Build()

	r := &DataStore{
		Client:  cl,
		Metrics: metrics.NewRecorder(prometheus.NewRegistry()),
	}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "primary"}}); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	var got kamajiv1alpha1.DataStore
	if err := cl.Get(t.Context(), types.NamespacedName{Name: "primary"}, &got); err != nil {
		t.Fatalf("failed getting DataStore: %v", err)
	}

	condition := meta.FindStatusCondition(got.Status.Conditions, kamajiv1alpha1.DataStoreConditionValidType)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected a true Valid condition, got %+v", condition)
	}
	if condition.ObservedGeneration != got.Generation {
		t.Fatalf("expected Valid condition observedGeneration %d, got %d", got.Generation, condition.ObservedGeneration)
	}
}
