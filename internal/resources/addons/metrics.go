// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	kubeProxyCollector prometheus.Histogram
	coreDNSCollector   prometheus.Histogram
)
