// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestDataStoreReconcilePredicateIgnoresStatusOnlyUpdates(t *testing.T) {
	t.Parallel()

	oldDS := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "default",
			Generation: 1,
		},
	}
	newDS := oldDS.DeepCopy()
	newDS.Status.UsedBy = []string{"default/tcp-a"}

	if (dataStoreReconcilePredicate{}).Update(event.TypedUpdateEvent[client.Object]{
		ObjectOld: oldDS,
		ObjectNew: newDS,
	}) {
		t.Fatal("expected DataStore status-only update to be ignored")
	}
}

func TestDataStoreReconcilePredicateAcceptsSpecAndDeletionUpdates(t *testing.T) {
	t.Parallel()

	oldDS := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "default",
			Generation: 1,
		},
	}
	newDS := oldDS.DeepCopy()
	newDS.Generation = 2

	if !(dataStoreReconcilePredicate{}).Update(event.TypedUpdateEvent[client.Object]{
		ObjectOld: oldDS,
		ObjectNew: newDS,
	}) {
		t.Fatal("expected DataStore generation update to be accepted")
	}

	deletingDS := oldDS.DeepCopy()
	now := metav1.Now()
	deletingDS.DeletionTimestamp = &now

	if !(dataStoreReconcilePredicate{}).Update(event.TypedUpdateEvent[client.Object]{
		ObjectOld: oldDS,
		ObjectNew: deletingDS,
	}) {
		t.Fatal("expected DataStore deletion update to be accepted")
	}
}

func TestDataStoreReconcileDoesNotOwnUsedByStatus(t *testing.T) {
	t.Parallel()

	scheme := newKamajiTestScheme(t)
	ds := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "default",
			Finalizers: []string{kamajiv1alpha1.DataStoreTCPFinalizer},
		},
		Status: kamajiv1alpha1.DataStoreStatus{
			UsedBy: []string{"stale/value"},
			Conditions: []metav1.Condition{{
				Type:   kamajiv1alpha1.DataStoreConditionValidType,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	tcp := tenantControlPlaneWithDataStoreStatus("default", "default")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ds, tcp).
		WithStatusSubresource(ds).
		WithIndex(&kamajiv1alpha1.TenantControlPlane{}, kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, (&kamajiv1alpha1.TenantControlPlaneStatusDataStore{}).ExtractValue()).
		Build()

	r := &DataStore{
		Client:                    c,
		TenantControlPlaneTrigger: make(chan event.GenericEvent, 1),
	}
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "default"}}); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	var updated kamajiv1alpha1.DataStore
	if err := c.Get(t.Context(), types.NamespacedName{Name: "default"}, &updated); err != nil {
		t.Fatalf("failed getting datastore: %v", err)
	}

	if !equalStringSet(updated.Status.UsedBy, []string{"stale/value"}) {
		t.Fatalf("expected DataStore controller to preserve usedBy, got %v", updated.Status.UsedBy)
	}
}
