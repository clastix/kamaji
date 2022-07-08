// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

const (
	AgentName      = "konnectivity-agent"
	CertCommonName = "system:konnectivity-server"

	agentTokenName                  = "konnectivity-agent-token"
	apiServerAPIVersion             = "apiserver.k8s.io/v1beta1"
	certExpirationDelayYears        = 10
	certOrganization                = "system:master"
	defaultClusterName              = "kubernetes"
	defaultUDSName                  = "/run/konnectivity/konnectivity-server.socket"
	egressSelectorConfigurationKind = "EgressSelectorConfiguration"
	egressSelectorConfigurationName = "cluster"
	konnectivityCertAndKeyBaseName  = "konnectivity"
	konnectivityKubeconfigFileName  = "konnectivity-server.conf"
	kubeconfigAPIVersion            = "v1"
	roleAuthDelegator               = "system:auth-delegator"
)
