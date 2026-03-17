// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/metrics"
)

func TestDataStoreRefreshMetrics(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&kamajiv1alpha1.DataStore{
			ObjectMeta: metav1.ObjectMeta{Name: "primary"},
			Spec:       kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.EtcdDriver},
			Status: kamajiv1alpha1.DataStoreStatus{
				Ready: true,
				Conditions: []metav1.Condition{{
					Type:   kamajiv1alpha1.DataStoreConditionValidType,
					Status: metav1.ConditionTrue,
				}},
			},
		},
		&kamajiv1alpha1.DataStore{
			ObjectMeta: metav1.ObjectMeta{Name: "secondary"},
			Spec:       kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.Driver("unsupported")},
			Status:     kamajiv1alpha1.DataStoreStatus{Ready: false},
		},
	).Build()

	registry := prometheus.NewRegistry()
	recorder := metrics.NewRecorder(registry)

	r := &DataStore{
		Client:  reader,
		Metrics: recorder,
	}

	if err := r.refreshDatastoreMetrics(t.Context()); err != nil {
		t.Fatalf("refreshDatastoreMetrics returned error: %v", err)
	}

	countFamily := mustMetricFamilyFromGatherer(t, registry, "kamaji_datastores_current")
	if got := gaugeValueByLabels(t, countFamily, map[string]string{"datastore_name": "primary", "ready": metrics.ReadyLabelTrue, "driver": string(kamajiv1alpha1.EtcdDriver)}); got != 1 {
		t.Fatalf("expected primary ready/etcd count to be 1, got %v", got)
	}
	if got := gaugeValueByLabels(t, countFamily, map[string]string{"datastore_name": "secondary", "ready": metrics.ReadyLabelFalse, "driver": metrics.DataStoreDriverUnknown}); got != 1 {
		t.Fatalf("expected secondary not-ready/unknown count to be 1, got %v", got)
	}

	statusFamily := mustMetricFamilyFromGatherer(t, registry, "kamaji_datastore_status")
	if got := gaugeValueByLabels(t, statusFamily, map[string]string{"datastore_name": "primary", "status": metrics.DataStoreStatusTrue, "ready": metrics.ReadyLabelTrue}); got != 1 {
		t.Fatalf("expected primary status metric to be 1, got %v", got)
	}
	if got := gaugeValueByLabels(t, statusFamily, map[string]string{"datastore_name": "secondary", "status": metrics.DataStoreStatusUnknown, "ready": metrics.ReadyLabelFalse}); got != 1 {
		t.Fatalf("expected secondary status metric to be 1, got %v", got)
	}
}

func mustMetricFamilyFromGatherer(t *testing.T, gatherer prometheus.Gatherer, name string) *io_prometheus_client.MetricFamily {
	t.Helper()

	families, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}

	t.Fatalf("metric family %s not found", name)

	return nil
}

func gaugeValueByLabels(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string) float64 {
	t.Helper()

	for _, metric := range family.GetMetric() {
		allMatch := true
		for name, value := range labels {
			matched := false
			for _, label := range metric.GetLabel() {
				if label.GetName() == name && label.GetValue() == value {
					matched = true

					break
				}
			}

			if !matched {
				allMatch = false

				break
			}
		}

		if allMatch {
			return metric.GetGauge().GetValue()
		}
	}

	t.Fatalf("metric with labels %v not found", labels)

	return 0
}
