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

func TestDataStoreUsageReconcileUpdatesUsedBy(t *testing.T) {
	t.Parallel()

	scheme := newKamajiTestScheme(t)
	ds := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}
	tcp := tenantControlPlaneWithDataStoreStatus("default", "default")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ds, tcp).
		WithStatusSubresource(ds).
		WithIndex(&kamajiv1alpha1.TenantControlPlane{}, kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, (&kamajiv1alpha1.TenantControlPlaneStatusDataStore{}).ExtractValue()).
		Build()

	r := &DataStoreUsage{Client: c}
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "default"}}); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	var updated kamajiv1alpha1.DataStore
	if err := c.Get(t.Context(), types.NamespacedName{Name: "default"}, &updated); err != nil {
		t.Fatalf("failed getting datastore: %v", err)
	}

	if !equalStringSet(updated.Status.UsedBy, []string{"default/tcp-a"}) {
		t.Fatalf("expected usedBy to contain default/tcp-a, got %v", updated.Status.UsedBy)
	}
}

func TestDataStoreUsageReconcileRefreshesOldAndNewDataStoreMembership(t *testing.T) {
	t.Parallel()

	scheme := newKamajiTestScheme(t)
	oldDS := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{Name: "old"},
		Status: kamajiv1alpha1.DataStoreStatus{
			UsedBy: []string{"default/tcp-a"},
		},
	}
	newDS := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{Name: "new"},
	}
	tcp := tenantControlPlaneWithDataStoreStatus("new", "new")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(oldDS, newDS, tcp).
		WithStatusSubresource(oldDS, newDS).
		WithIndex(&kamajiv1alpha1.TenantControlPlane{}, kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, (&kamajiv1alpha1.TenantControlPlaneStatusDataStore{}).ExtractValue()).
		Build()

	r := &DataStoreUsage{Client: c}
	for _, dataStoreName := range []string{"old", "new"} {
		if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dataStoreName}}); err != nil {
			t.Fatalf("reconcile for %s returned error: %v", dataStoreName, err)
		}
	}

	var updatedOld kamajiv1alpha1.DataStore
	if err := c.Get(t.Context(), types.NamespacedName{Name: "old"}, &updatedOld); err != nil {
		t.Fatalf("failed getting old datastore: %v", err)
	}

	if len(updatedOld.Status.UsedBy) != 0 {
		t.Fatalf("expected old datastore usedBy to be empty, got %v", updatedOld.Status.UsedBy)
	}

	var updatedNew kamajiv1alpha1.DataStore
	if err := c.Get(t.Context(), types.NamespacedName{Name: "new"}, &updatedNew); err != nil {
		t.Fatalf("failed getting new datastore: %v", err)
	}

	if !equalStringSet(updatedNew.Status.UsedBy, []string{"default/tcp-a"}) {
		t.Fatalf("expected new datastore usedBy to contain default/tcp-a, got %v", updatedNew.Status.UsedBy)
	}
}

func TestDataStoreUsageUpdatePredicateIgnoresStatusOnlyChanges(t *testing.T) {
	t.Parallel()

	oldTCP := tenantControlPlaneWithDataStoreStatus("default", "default")
	newTCP := oldTCP.DeepCopy()
	newTCP.Status.Kubernetes.Deployment.LastUpdate = metav1.Now()

	if shouldEnqueueDataStoreUsageUpdate(oldTCP, newTCP) {
		t.Fatal("expected status-only update to be ignored")
	}
}

func TestDataStoreUsageUpdatePredicateIgnoresSpecOnlyChanges(t *testing.T) {
	t.Parallel()

	oldTCP := tenantControlPlaneWithDataStoreStatus("old", "default")
	newTCP := tenantControlPlaneWithDataStoreStatus("new", "default")

	if shouldEnqueueDataStoreUsageUpdate(oldTCP, newTCP) {
		t.Fatal("expected spec-only datastore update to be ignored")
	}
}

func TestDataStoreUsageUpdatePredicateAcceptsStorageStatusChanges(t *testing.T) {
	t.Parallel()

	oldTCP := tenantControlPlaneWithDataStoreStatus("old", "old")
	newTCP := tenantControlPlaneWithDataStoreStatus("new", "new")

	if !shouldEnqueueDataStoreUsageUpdate(oldTCP, newTCP) {
		t.Fatal("expected datastore membership update to be accepted")
	}
}

func TestDataStoreUsagePredicateAcceptsCreateEventsOnly(t *testing.T) {
	t.Parallel()

	ds := &kamajiv1alpha1.DataStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	updatedDS := ds.DeepCopy()
	updatedDS.Status.UsedBy = []string{"default/tcp-a"}

	p := dataStoreUsagePredicate{}
	if !p.Create(event.TypedCreateEvent[client.Object]{Object: ds}) {
		t.Fatal("expected DataStore create event to be accepted")
	}
	if p.Delete(event.TypedDeleteEvent[client.Object]{Object: ds}) {
		t.Fatal("expected DataStore delete event to be ignored")
	}
	if p.Update(event.TypedUpdateEvent[client.Object]{ObjectOld: ds, ObjectNew: updatedDS}) {
		t.Fatal("expected DataStore update event to be ignored")
	}
	if p.Generic(event.TypedGenericEvent[client.Object]{Object: ds}) {
		t.Fatal("expected DataStore generic event to be ignored")
	}
}
