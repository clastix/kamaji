// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	agentCollector              prometheus.Histogram
	certificateCollector        prometheus.Histogram
	clusterrolebindingCollector prometheus.Histogram
	deploymentCollector         prometheus.Histogram
	egressCollector             prometheus.Histogram
	gatewayCollector            prometheus.Histogram
	kubeconfigCollector         prometheus.Histogram
	serviceaccountCollector     prometheus.Histogram
	serviceCollector            prometheus.Histogram
)
