// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestHandlerHistogramUsesSingleMetricWithHandlerLabel(t *testing.T) {
	handlerCollector.Reset()

	first := fakeResource{name: "test_handler_a"}
	second := fakeResource{name: "test_handler_b"}

	LazyLoadHistogramFromResource(nil, first).Observe(0.10)
	LazyLoadHistogramFromResource(nil, second).Observe(0.20)

	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metric families: %v", err)
	}

	var (
		foundHandlerFamily bool
		foundLegacyName    bool
	)

	for _, family := range families {
		name := family.GetName()
		if name == "kamaji_handler_time_seconds" {
			foundHandlerFamily = true

			if !hasLabelValueForFamily(family, "handler", "test_handler_a") {
				t.Fatalf("expected handler label value test_handler_a in kamaji_handler_time_seconds")
			}

			if !hasLabelValueForFamily(family, "handler", "test_handler_b") {
				t.Fatalf("expected handler label value test_handler_b in kamaji_handler_time_seconds")
			}
		}

		if name == "kamaji_handler_test_handler_a_time_seconds" || name == "kamaji_handler_test_handler_b_time_seconds" {
			foundLegacyName = true
		}
	}

	if !foundHandlerFamily {
		t.Fatalf("metric family kamaji_handler_time_seconds not found")
	}

	if foundLegacyName {
		t.Fatalf("unexpected legacy per-handler metric family found")
	}
}

func hasLabelValueForFamily(family *io_prometheus_client.MetricFamily, labelName, labelValue string) bool {
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == labelName && label.GetValue() == labelValue {
				return true
			}
		}
	}

	return false
}

type fakeResource struct {
	name string
}

func (f fakeResource) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (f fakeResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (f fakeResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (f fakeResource) CreateOrUpdate(context.Context, *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.OperationResultNone, nil
}

func (f fakeResource) GetName() string {
	return f.name
}

func (f fakeResource) ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (f fakeResource) UpdateTenantControlPlaneStatus(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (f fakeResource) GetHistogram() prometheus.Histogram {
	return LazyLoadHistogramFromResource(nil, f)
}
