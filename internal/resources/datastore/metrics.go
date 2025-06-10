// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	certificateCollector  prometheus.Histogram
	migrateCollector      prometheus.Histogram
	multiTenancyCollector prometheus.Histogram
	setupCollector        prometheus.Histogram
	storageCollector      prometheus.Histogram
)
