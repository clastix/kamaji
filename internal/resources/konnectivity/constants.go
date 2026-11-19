// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"k8s.io/kubernetes/pkg/apis/core"
)

const (
	AgentName      = "konnectivity-agent"
	CertCommonName = "system:konnectivity-server"
	AgentNamespace = core.NamespaceSystem

	agentHealthServerPort           = 8134
	agentTokenName                  = "konnectivity-agent-token"
	apiServerAPIVersion             = "apiserver.k8s.io/v1beta1"
	defaultClusterName              = "kubernetes"
	defaultUDSName                  = "/run/konnectivity/konnectivity-server.socket"
	egressSelectorConfigurationKind = "EgressSelectorConfiguration"
	egressSelectorConfigurationName = "cluster"
	konnectivityCertAndKeyBaseName  = "konnectivity"
	konnectivityKubeconfigFileName  = "konnectivity-server.conf"
	kubeconfigAPIVersion            = "v1"
	roleAuthDelegator               = "system:auth-delegator"
)

// agentKeepaliveTime is how long the agent leaves a tunnel idle before it
// pings the server. With the upstream default of 1h, a tunnel whose
// network path changed stays in use, dead, for up to an hour. It must
// stay at least the server's keepalive enforcement minimum (30s).
const agentKeepaliveTime = "60s"
