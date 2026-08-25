// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"

	"github.com/stretchr/testify/require"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/kubernetes/cmd/kubeadm/app/componentconfigs"
)

func TestGetKubeadmInitConfigurationFromMapDefaultsKubeProxyClusterCIDR(t *testing.T) {
	testCases := []struct {
		name     string
		podCIDRs []string
		expected string
	}{
		{
			name:     "IPv4",
			podCIDRs: []string{"10.244.0.0/16"},
			expected: "10.244.0.0/16",
		},
		{
			name:     "dual stack",
			podCIDRs: []string{"10.244.0.0/16", "fd00:10:244::/56"},
			expected: "10.244.0.0/16,fd00:10:244::/56",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			created, err := CreateKubeadmInitConfiguration(Parameters{
				TenantControlPlaneName:          "tenant",
				TenantControlPlaneNamespace:     "default",
				TenantControlPlaneAddress:       "192.0.2.1",
				TenantControlPlanePort:          6443,
				TenantControlPlaneClusterDomain: "cluster.local",
				TenantControlPlanePodCIDR:       testCase.podCIDRs,
				TenantControlPlaneServiceCIDR:   []string{"10.96.0.0/12"},
				TenantControlPlaneVersion:       "v1.36.1",
				ETCDs:                           []string{"http://etcd.default.svc:2379"},
			})
			require.NoError(t, err)

			stored, err := GetKubeadmInitConfigurationMap(*created)
			require.NoError(t, err)

			restored, err := GetKubeadmInitConfigurationFromMap(stored)
			require.NoError(t, err)

			componentConfig, ok := restored.InitConfiguration.ClusterConfiguration.ComponentConfigs[componentconfigs.KubeProxyGroup]
			require.True(t, ok)

			proxyConfig, ok := componentConfig.Get().(*kubeproxyconfig.KubeProxyConfiguration)
			require.True(t, ok)
			require.Equal(t, testCase.expected, proxyConfig.ClusterCIDR)
		})
	}
}
