// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	apiservercertificateCollector      prometheus.Histogram
	clientcertificateCollector         prometheus.Histogram
	certificateauthorityCollector      prometheus.Histogram
	frontproxycertificateCollector     prometheus.Histogram
	frontproxycaCollector              prometheus.Histogram
	deploymentCollector                prometheus.Histogram
	ingressCollector                   prometheus.Histogram
	gatewayCollector                   prometheus.Histogram
	serviceCollector                   prometheus.Histogram
	kubeadmconfigCollector             prometheus.Histogram
	kubeadmupgradeCollector            prometheus.Histogram
	kubeconfigCollector                prometheus.Histogram
	serviceaccountcertificateCollector prometheus.Histogram

	kubeadmphaseUploadConfigKubeadmCollector prometheus.Histogram
	kubeadmphaseUploadConfigKubeletCollector prometheus.Histogram
	kubeadmphaseBootstrapTokenCollector      prometheus.Histogram
	kubeadmphaseClusterAdminRBACCollector    prometheus.Histogram
)

func LazyLoadHistogramFromResource(collector prometheus.Histogram, resource Resource) prometheus.Histogram {
	n := resource.GetName()

	if collector == nil {
		c := prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "kamaji",
			Subsystem: "handler",
			Name:      n + "_time_seconds",
			Help:      "Bucket time requested for the given handler to complete its handling.",
			Buckets: []float64{
				0.005, 0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0,
				1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5, 6, 7, 8, 9, 10, 15, 20, 25, 30, 40, 50, 60,
			},
		})

		metrics.Registry.MustRegister(c)

		return c
	}

	return collector
}
