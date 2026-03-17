// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kamajiversion "github.com/clastix/kamaji/internal"
)

const (
	ReadyLabelTrue  = "true"
	ReadyLabelFalse = "false"

	DataStoreDriverUnknown = "unknown"

	DataStoreStatusUnknown = "Unknown"
	DataStoreStatusTrue    = "True"
	DataStoreStatusFalse   = "False"

	TenantControlPlaneExposureService = "service"
	TenantControlPlaneExposureGateway = "gateway"
	TenantControlPlaneExposureUnknown = "unknown"

	TenantControlPlaneKubernetesVersionUnknown = "unknown"

	CertificateStatusValid    = "valid"
	CertificateStatusExpiring = "expiring"
	CertificateStatusInvalid  = "invalid"

	CertificateStrategyX509       = "x509"
	CertificateStrategyKubeconfig = "kubeconfig"

	kamajiNamespace = "kamaji"

	buildSubsystem              = "build"
	tenantControlPlaneSubsystem = "tenant_control_plane"
	tenantControlPlanesS        = "tenant_control_planes"
	datastoreSubsystem          = "datastore"
	datastoresSubsystem         = "datastores"
	certificatesSubsystem       = "certificates"

	metricNameInfo    = "info"
	metricNameStatus  = "status"
	metricNameCurrent = "current"

	labelTCPNamespace     = "tcp_namespace"
	labelTCPName          = "tcp_name"
	labelKubernetesVer    = "kubernetes_version"
	labelDatastoreDriver  = "datastore_driver"
	labelAddress          = "address"
	labelExposureStrategy = "exposure_strategy"
	labelStatus           = "status"
	labelReady            = "ready"
	labelDataStoreName    = "datastore_name"
	labelDriver           = "driver"
	labelStrategy         = "strategy"
	labelVersion          = "version"
	labelCommit           = "commit"
	labelDirty            = "dirty"
	labelRepo             = "repo"
	labelBuildTime        = "build_time"

	metricsRefreshTimeout = 5 * time.Second
)

func NewRefreshContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), metricsRefreshTimeout)
}

func NewRefreshContextFrom(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithTimeout(ctx, metricsRefreshTimeout)
}

var readyLabelValues = []string{
	ReadyLabelTrue,
	ReadyLabelFalse,
}

var tenantControlPlaneStatuses = []string{
	string(kamajiv1alpha1.VersionUnknown),
	string(kamajiv1alpha1.VersionProvisioning),
	string(kamajiv1alpha1.VersionCARotating),
	string(kamajiv1alpha1.VersionUpgrading),
	string(kamajiv1alpha1.VersionMigrating),
	string(kamajiv1alpha1.VersionReady),
	string(kamajiv1alpha1.VersionNotReady),
	string(kamajiv1alpha1.VersionSleeping),
	string(kamajiv1alpha1.VersionWriteLimited),
}

var datastoreDrivers = []string{
	string(kamajiv1alpha1.EtcdDriver),
	string(kamajiv1alpha1.KineMySQLDriver),
	string(kamajiv1alpha1.KinePostgreSQLDriver),
	string(kamajiv1alpha1.KineNatsDriver),
	DataStoreDriverUnknown,
}

var certificateStatuses = []string{
	CertificateStatusValid,
	CertificateStatusExpiring,
	CertificateStatusInvalid,
}

var certificateStrategies = []string{
	CertificateStrategyX509,
	CertificateStrategyKubeconfig,
}

var (
	defaultRecorderOnce sync.Once
	defaultRecorder     *Recorder
)

type Recorder struct {
	buildInfo                *prometheus.GaugeVec
	tenantControlPlaneInfo   *prometheus.GaugeVec
	tenantControlPlaneStatus *prometheus.GaugeVec
	datastoreInfo            *prometheus.GaugeVec
	datastoreStatus          *prometheus.GaugeVec
	controlPlanesCount       *prometheus.GaugeVec
	datastoresCount          *prometheus.GaugeVec
	certificatesCount        *prometheus.GaugeVec
}

func DefaultRecorder() *Recorder {
	defaultRecorderOnce.Do(func() {
		defaultRecorder = NewRecorder(ctrlmetrics.Registry)
	})

	return defaultRecorder
}

func NewRecorder(registerer prometheus.Registerer) *Recorder {
	if registerer == nil {
		registerer = ctrlmetrics.Registry
	}

	recorder := &Recorder{
		buildInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: buildSubsystem,
			Name:      metricNameInfo,
			Help:      "Build information for Kamaji.",
		}, []string{labelVersion, labelCommit, labelDirty, labelRepo, labelBuildTime}),
		tenantControlPlaneInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: tenantControlPlaneSubsystem,
			Name:      metricNameInfo,
			Help:      "Identity and high-level configuration information for TenantControlPlane resources.",
		}, []string{labelTCPNamespace, labelTCPName, labelKubernetesVer, labelDatastoreDriver, labelAddress, labelExposureStrategy}),
		tenantControlPlaneStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: tenantControlPlaneSubsystem,
			Name:      metricNameStatus,
			Help:      "Current status and readiness of TenantControlPlane resources.",
		}, []string{labelTCPNamespace, labelTCPName, labelStatus, labelReady}),
		datastoreInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: datastoreSubsystem,
			Name:      metricNameInfo,
			Help:      "Identity and high-level configuration information for DataStore resources.",
		}, []string{labelDataStoreName, labelDriver}),
		datastoreStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: datastoreSubsystem,
			Name:      metricNameStatus,
			Help:      "Current status and readiness of DataStore resources.",
		}, []string{labelDataStoreName, labelStatus, labelReady}),
		controlPlanesCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: tenantControlPlanesS,
			Name:      metricNameCurrent,
			Help:      "Current number of TenantControlPlane resources by ready flag.",
		}, []string{labelReady}),
		datastoresCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: datastoresSubsystem,
			Name:      metricNameCurrent,
			Help:      "Current number of DataStore resources by name, ready flag and driver.",
		}, []string{labelDataStoreName, labelReady, labelDriver}),
		certificatesCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: kamajiNamespace,
			Subsystem: certificatesSubsystem,
			Name:      metricNameCurrent,
			Help:      "Current number of certificate Secrets by tcp_namespace, tcp_name, status and strategy.",
		}, []string{labelTCPNamespace, labelTCPName, labelStatus, labelStrategy}),
	}

	registerer.MustRegister(
		recorder.buildInfo,
		recorder.tenantControlPlaneInfo,
		recorder.tenantControlPlaneStatus,
		recorder.datastoreInfo,
		recorder.datastoreStatus,
		recorder.controlPlanesCount,
		recorder.datastoresCount,
		recorder.certificatesCount,
	)

	recorder.buildInfo.WithLabelValues(
		kamajiversion.GitTag,
		kamajiversion.GitCommit,
		kamajiversion.GitDirty,
		kamajiversion.GitRepo,
		kamajiversion.BuildTime,
	).Set(1)

	recorder.SetTenantControlPlanesReadyCounts(NewReadyCounts())

	return recorder
}

func NewReadyCounts() map[string]int {
	return map[string]int{
		ReadyLabelTrue:  0,
		ReadyLabelFalse: 0,
	}
}

func NewSingleDriverReadyCounts(driver string) map[string]map[string]int {
	return map[string]map[string]int{
		ReadyLabelTrue: {
			driver: 0,
		},
		ReadyLabelFalse: {
			driver: 0,
		},
	}
}

func NewCertificateStatusCounts() map[string]map[string]int {
	return map[string]map[string]int{
		CertificateStatusValid: {
			CertificateStrategyX509:       0,
			CertificateStrategyKubeconfig: 0,
		},
		CertificateStatusExpiring: {
			CertificateStrategyX509:       0,
			CertificateStrategyKubeconfig: 0,
		},
		CertificateStatusInvalid: {
			CertificateStrategyX509:       0,
			CertificateStrategyKubeconfig: 0,
		},
	}
}

func (r *Recorder) SetTenantControlPlanesReadyCounts(counts map[string]int) {
	for _, ready := range readyLabelValues {
		r.controlPlanesCount.WithLabelValues(ready).Set(float64(counts[ready]))
	}
}

func (r *Recorder) ResetTenantControlPlaneStatus() {
	r.tenantControlPlaneStatus.Reset()
}

func (r *Recorder) SetTenantControlPlaneStatus(tcpNamespace, tcpName, status, ready string) {
	r.tenantControlPlaneStatus.WithLabelValues(tcpNamespace, tcpName, status, ready).Set(1)
}

func (r *Recorder) ResetDataStoreStatus() {
	r.datastoreStatus.Reset()
}

func (r *Recorder) SetDataStoreStatus(datastoreName, status, ready string) {
	r.datastoreStatus.WithLabelValues(datastoreName, status, ready).Set(1)
}

func (r *Recorder) ResetTenantControlPlaneInfo() {
	r.tenantControlPlaneInfo.Reset()
}

func (r *Recorder) SetTenantControlPlaneInfo(tcpNamespace, tcpName, kubernetesVersion, dataStoreDriver, address, exposureStrategy string) {
	r.tenantControlPlaneInfo.WithLabelValues(
		tcpNamespace,
		tcpName,
		kubernetesVersion,
		dataStoreDriver,
		address,
		exposureStrategy,
	).Set(1)
}

func (r *Recorder) ResetDataStoreInfo() {
	r.datastoreInfo.Reset()
}

func (r *Recorder) SetDataStoreInfo(datastoreName, dataStoreDriver string) {
	r.datastoreInfo.WithLabelValues(datastoreName, dataStoreDriver).Set(1)
}

func NormalizeTenantControlPlaneStatusLabel(status *kamajiv1alpha1.KubernetesVersionStatus) string {
	if status == nil {
		return string(kamajiv1alpha1.VersionUnknown)
	}

	value := string(*status)
	for _, allowedStatus := range tenantControlPlaneStatuses {
		if value == allowedStatus {
			return value
		}
	}

	return string(kamajiv1alpha1.VersionUnknown)
}

func (r *Recorder) ResetDatastoresReadyAndDriverCounts() {
	r.datastoresCount.Reset()
}

func (r *Recorder) SetDatastoresReadyAndDriverCounts(datastoreName string, counts map[string]map[string]int) {
	for ready, readyCounts := range counts {
		for driver, value := range readyCounts {
			r.datastoresCount.WithLabelValues(datastoreName, ready, driver).Set(float64(value))
		}
	}
}

func NormalizeDataStoreDriverLabel(driver kamajiv1alpha1.Driver) string {
	for _, knownDriver := range datastoreDrivers {
		if string(driver) == knownDriver {
			return string(driver)
		}
	}

	return DataStoreDriverUnknown
}

func NormalizeDataStoreDriverStringLabel(driver string) string {
	return NormalizeDataStoreDriverLabel(kamajiv1alpha1.Driver(driver))
}

func NormalizeDataStoreConditionStatusLabel(status metav1.ConditionStatus) string {
	switch status {
	case metav1.ConditionTrue:
		return DataStoreStatusTrue
	case metav1.ConditionFalse:
		return DataStoreStatusFalse
	default:
		return DataStoreStatusUnknown
	}
}

func NormalizeTenantControlPlaneKubernetesVersionLabel(version string) string {
	if version == "" {
		return TenantControlPlaneKubernetesVersionUnknown
	}

	return version
}

func (r *Recorder) ResetCertificatesStatusCounts() {
	r.certificatesCount.Reset()
}

func (r *Recorder) SetCertificatesStatusCounts(tcpNamespace, tcpName string, counts map[string]map[string]int) {
	for _, status := range certificateStatuses {
		for _, strategy := range certificateStrategies {
			value := 0
			if statusCounts, ok := counts[status]; ok {
				value = statusCounts[strategy]
			}

			r.certificatesCount.WithLabelValues(tcpNamespace, tcpName, status, strategy).Set(float64(value))
		}
	}
}
