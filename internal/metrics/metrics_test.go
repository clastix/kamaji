// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func testRecorder() *Recorder {
	return DefaultRecorder()
}

func TestKamajiMetricRegistration(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetDatastoresReadyAndDriverCounts()
	recorder.SetDatastoresReadyAndDriverCounts("default", map[string]map[string]int{
		ReadyLabelTrue: {
			string(kamajiv1alpha1.EtcdDriver): 1,
		},
	})

	recorder.ResetCertificatesStatusCounts()
	recorder.SetCertificatesStatusCounts("default", "k8s-133", map[string]map[string]int{
		CertificateStatusValid: {
			CertificateStrategyX509: 1,
		},
	})

	wanted := map[string]struct{}{
		"kamaji_build_info":                    {},
		"kamaji_tenant_control_planes_current": {},
		"kamaji_datastores_current":            {},
		"kamaji_certificates_current":          {},
	}

	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	seen := map[string]struct{}{}
	for _, family := range families {
		if _, ok := wanted[family.GetName()]; ok {
			seen[family.GetName()] = struct{}{}
		}
	}

	for name := range wanted {
		if _, ok := seen[name]; !ok {
			t.Fatalf("metric %s not registered", name)
		}
	}
}

func TestNewRecorderWithCustomRegistry(t *testing.T) {
	t.Helper()

	registry := prometheus.NewRegistry()
	recorder := NewRecorder(registry)
	recorder.SetTenantControlPlanesReadyCounts(map[string]int{
		ReadyLabelTrue:  1,
		ReadyLabelFalse: 0,
	})

	family := mustMetricFamilyFromGatherer(t, registry, "kamaji_tenant_control_planes_current")
	if got := gaugeValueByLabels(t, family, map[string]string{"ready": ReadyLabelTrue}); got != 1 {
		t.Fatalf("expected ready control planes gauge to be 1 on custom registry, got %v", got)
	}
}

func TestTenantControlPlanesCountGaugeByStatus(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.SetTenantControlPlanesReadyCounts(map[string]int{
		ReadyLabelTrue:  2,
		ReadyLabelFalse: 1,
	})

	family := mustMetricFamily(t, "kamaji_tenant_control_planes_current")
	if len(family.GetMetric()) != len(readyLabelValues) {
		t.Fatalf("expected %d ready-only gauge metrics, got %d", len(readyLabelValues), len(family.GetMetric()))
	}

	if got := gaugeValueByLabels(t, family, map[string]string{"ready": ReadyLabelTrue}); got != 2 {
		t.Fatalf("expected ready control planes gauge to be 2, got %v", got)
	}

	if got := gaugeValueByLabels(t, family, map[string]string{"ready": ReadyLabelFalse}); got != 1 {
		t.Fatalf("expected not-ready control planes gauge to be 1, got %v", got)
	}
}

func TestDatastoresCountGaugeByStatus(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetDatastoresReadyAndDriverCounts()

	recorder.SetDatastoresReadyAndDriverCounts("default", map[string]map[string]int{
		ReadyLabelTrue: {
			string(kamajiv1alpha1.EtcdDriver): 1,
		},
		ReadyLabelFalse: {
			string(kamajiv1alpha1.EtcdDriver): 0,
		},
	})
	recorder.SetDatastoresReadyAndDriverCounts("secondary", map[string]map[string]int{
		ReadyLabelTrue: {
			string(kamajiv1alpha1.KineMySQLDriver): 1,
		},
		ReadyLabelFalse: {
			string(kamajiv1alpha1.KineMySQLDriver): 0,
		},
	})

	family := mustMetricFamily(t, "kamaji_datastores_current")
	if len(family.GetMetric()) != 4 {
		t.Fatalf("expected 4 name-ready-driver gauge metrics, got %d", len(family.GetMetric()))
	}

	assertMetricFamilyHasLabels(t, family, "datastore_name", "ready", "driver")
	assertMetricFamilyDoesNotHaveLabels(t, family, "namespace")

	if got := gaugeValueByLabels(t, family, map[string]string{"datastore_name": "default", "ready": ReadyLabelTrue, "driver": string(kamajiv1alpha1.EtcdDriver)}); got != 1 {
		t.Fatalf("expected ready datastores gauge to be 1, got %v", got)
	}

	if got := gaugeValueByLabels(t, family, map[string]string{"datastore_name": "default", "ready": ReadyLabelFalse, "driver": string(kamajiv1alpha1.EtcdDriver)}); got != 0 {
		t.Fatalf("expected not_ready/etcd datastores gauge to be 0, got %v", got)
	}

	if got := gaugeValueByLabels(t, family, map[string]string{"datastore_name": "secondary", "ready": ReadyLabelTrue, "driver": string(kamajiv1alpha1.KineMySQLDriver)}); got != 1 {
		t.Fatalf("expected secondary/mysql ready datastores gauge to be 1, got %v", got)
	}

	if got := gaugeValueByLabels(t, family, map[string]string{"datastore_name": "secondary", "ready": ReadyLabelFalse, "driver": string(kamajiv1alpha1.KineMySQLDriver)}); got != 0 {
		t.Fatalf("expected secondary/mysql not-ready datastores gauge to be 0, got %v", got)
	}
}

func TestDataStoreInfoMetric(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetDataStoreInfo()
	recorder.SetDataStoreInfo("default", string(kamajiv1alpha1.EtcdDriver))

	family := mustMetricFamily(t, "kamaji_datastore_info")
	if len(family.GetMetric()) != 1 {
		t.Fatalf("expected one datastore_info sample, got %d", len(family.GetMetric()))
	}

	labels := map[string]string{
		"datastore_name": "default",
		"driver":         string(kamajiv1alpha1.EtcdDriver),
	}

	assertMetricFamilyHasLabels(t, family, "datastore_name", "driver")
	assertMetricFamilyDoesNotHaveLabels(t, family, "tls", "basic_auth")

	if got := gaugeValueByLabels(t, family, labels); got != 1 {
		t.Fatalf("expected datastore_info value to be 1, got %v", got)
	}
}

func TestDataStoreStatusMetric(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetDataStoreStatus()
	recorder.SetDataStoreStatus("default", DataStoreStatusTrue, ReadyLabelTrue)

	family := mustMetricFamily(t, "kamaji_datastore_status")
	if len(family.GetMetric()) != 1 {
		t.Fatalf("expected one datastore_status sample, got %d", len(family.GetMetric()))
	}

	labels := map[string]string{
		"datastore_name": "default",
		"status":         DataStoreStatusTrue,
		"ready":          ReadyLabelTrue,
	}

	if got := gaugeValueByLabels(t, family, labels); got != 1 {
		t.Fatalf("expected datastore_status value to be 1, got %v", got)
	}
}

func TestBuildInfoMetricHasSingleSample(t *testing.T) {
	t.Helper()

	family := mustMetricFamily(t, "kamaji_build_info")
	if len(family.GetMetric()) != 1 {
		t.Fatalf("expected a single build info metric, got %d", len(family.GetMetric()))
	}

	if got := family.GetMetric()[0].GetGauge().GetValue(); got != 1 {
		t.Fatalf("expected build info gauge to be 1, got %v", got)
	}
}

func TestTenantControlPlaneInfoMetric(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetTenantControlPlaneInfo()
	recorder.SetTenantControlPlaneInfo("default", "test", "v1.33.0", string(kamajiv1alpha1.EtcdDriver), "https://203.0.113.10:6443", TenantControlPlaneExposureService)

	family := mustMetricFamily(t, "kamaji_tenant_control_plane_info")
	if len(family.GetMetric()) != 1 {
		t.Fatalf("expected one tenant_control_plane_info sample, got %d", len(family.GetMetric()))
	}

	labels := map[string]string{
		"tcp_namespace":      "default",
		"tcp_name":           "test",
		"kubernetes_version": "v1.33.0",
		"datastore_driver":   string(kamajiv1alpha1.EtcdDriver),
		"address":            "https://203.0.113.10:6443",
		"exposure_strategy":  TenantControlPlaneExposureService,
	}

	assertMetricFamilyHasLabels(t, family, "tcp_namespace", "tcp_name", "kubernetes_version", "datastore_driver", "address", "exposure_strategy")
	assertMetricFamilyDoesNotHaveLabels(t, family, "namespace", "name")

	if got := gaugeValueByLabels(t, family, labels); got != 1 {
		t.Fatalf("expected tenant_control_plane_info value to be 1, got %v", got)
	}
}

func TestTenantControlPlaneStatusMetric(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetTenantControlPlaneStatus()
	recorder.SetTenantControlPlaneStatus("default", "test", string(kamajiv1alpha1.VersionReady), ReadyLabelTrue)

	family := mustMetricFamily(t, "kamaji_tenant_control_plane_status")
	if len(family.GetMetric()) != 1 {
		t.Fatalf("expected one tenant_control_plane_status sample, got %d", len(family.GetMetric()))
	}

	labels := map[string]string{
		"tcp_namespace": "default",
		"tcp_name":      "test",
		"status":        string(kamajiv1alpha1.VersionReady),
		"ready":         ReadyLabelTrue,
	}

	assertMetricFamilyHasLabels(t, family, "tcp_namespace", "tcp_name", "status", "ready")
	assertMetricFamilyDoesNotHaveLabels(t, family, "namespace", "name")

	if got := gaugeValueByLabels(t, family, labels); got != 1 {
		t.Fatalf("expected tenant_control_plane_status value to be 1, got %v", got)
	}
}

func TestCertificatesCountGaugeByStatusAndStrategy(t *testing.T) {
	t.Helper()
	recorder := testRecorder()

	recorder.ResetCertificatesStatusCounts()

	recorder.SetCertificatesStatusCounts("default", "k8s-133", map[string]map[string]int{
		CertificateStatusValid: {
			CertificateStrategyX509:       4,
			CertificateStrategyKubeconfig: 1,
		},
		CertificateStatusExpiring: {
			CertificateStrategyX509:       2,
			CertificateStrategyKubeconfig: 0,
		},
		CertificateStatusInvalid: {
			CertificateStrategyX509:       1,
			CertificateStrategyKubeconfig: 3,
		},
	})

	family := mustMetricFamily(t, "kamaji_certificates_current")
	if len(family.GetMetric()) != len(certificateStatuses)*len(certificateStrategies) {
		t.Fatalf("expected %d tcp_namespace-tcp_name-status-strategy gauge metrics, got %d", len(certificateStatuses)*len(certificateStrategies), len(family.GetMetric()))
	}

	assertMetricFamilyHasLabels(t, family, "tcp_namespace", "tcp_name", "status", "strategy")
	assertMetricFamilyDoesNotHaveLabels(t, family, "namespace", "name")

	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "k8s-133", "status": CertificateStatusValid, "strategy": CertificateStrategyX509}); got != 4 {
		t.Fatalf("expected valid/x509 certificates gauge to be 4, got %v", got)
	}

	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "k8s-133", "status": CertificateStatusInvalid, "strategy": CertificateStrategyKubeconfig}); got != 3 {
		t.Fatalf("expected invalid/kubeconfig certificates gauge to be 3, got %v", got)
	}
}

func assertMetricFamilyHasLabels(t *testing.T, family *io_prometheus_client.MetricFamily, labels ...string) {
	t.Helper()

	if len(family.GetMetric()) == 0 {
		t.Fatalf("metric family %s has no samples", family.GetName())
	}

	present := map[string]struct{}{}
	for _, label := range family.GetMetric()[0].GetLabel() {
		present[label.GetName()] = struct{}{}
	}

	for _, expected := range labels {
		if _, ok := present[expected]; !ok {
			t.Fatalf("metric family %s missing label %s", family.GetName(), expected)
		}
	}
}

func assertMetricFamilyDoesNotHaveLabels(t *testing.T, family *io_prometheus_client.MetricFamily, labels ...string) {
	t.Helper()

	if len(family.GetMetric()) == 0 {
		t.Fatalf("metric family %s has no samples", family.GetName())
	}

	present := map[string]struct{}{}
	for _, label := range family.GetMetric()[0].GetLabel() {
		present[label.GetName()] = struct{}{}
	}

	for _, forbidden := range labels {
		if _, ok := present[forbidden]; ok {
			t.Fatalf("metric family %s unexpectedly contains label %s", family.GetName(), forbidden)
		}
	}
}

func mustMetricFamily(t *testing.T, name string) *io_prometheus_client.MetricFamily {
	t.Helper()

	return mustMetricFamilyFromGatherer(t, metrics.Registry, name)
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
