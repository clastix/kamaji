// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapToken_ClusterInfoGeneration(t *testing.T) {
	caCert := []byte("dummy-ca-cert")
	defaultServerURL := "https://192.168.1.100:6443"
	publicServerURL := "https://k8s-api.example.com:6443"

	tests := []struct {
		name              string
		publicAddress     string
		port              int32
		expectedServerURL string
		description       string
	}{
		{
			name:              "uses kubeconfig server when no public address",
			publicAddress:     "",
			port:              6443,
			expectedServerURL: defaultServerURL,
			description:       "Should fall back to kubeconfig server URL when PublicAPIServerAddress is empty",
		},
		{
			name:              "uses public address with default port",
			publicAddress:     "k8s-api.example.com",
			port:              6443,
			expectedServerURL: publicServerURL,
			description:       "Should use public address with specified port",
		},
		{
			name:              "uses public address with custom port",
			publicAddress:     "k8s-api.example.com",
			port:              8443,
			expectedServerURL: "https://k8s-api.example.com:8443",
			description:       "Should use public address with custom port",
		},
		{
			name:              "uses public address with zero port (defaults to 6443)",
			publicAddress:     "k8s-api.example.com",
			port:              0,
			expectedServerURL: publicServerURL,
			description:       "Should default to port 6443 when port is zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake Kubernetes client
			client := fake.NewSimpleClientset()

			// Create test kubeconfig
			kubeconfig := clientcmdapiv1.Config{
				Clusters: []clientcmdapiv1.NamedCluster{
					{
						Cluster: clientcmdapiv1.Cluster{
							Server:                   defaultServerURL,
							CertificateAuthorityData: caCert,
						},
					},
				},
			}

			// Create test configuration
			config := &Configuration{
				Kubeconfig: kubeconfig,
				Parameters: Parameters{
					TenantControlPlanePublicAddress: tt.publicAddress,
					TenantControlPlanePort:          tt.port,
				},
			}

			// Call the BootstrapToken function
			err := BootstrapToken(client, config)
			require.NoError(t, err, "BootstrapToken should not return an error")

			// Verify the cluster-info ConfigMap was created
			cm, err := client.CoreV1().ConfigMaps(metav1.NamespacePublic).Get(
				nil,
				bootstrapapi.ConfigMapClusterInfo,
				metav1.GetOptions{},
			)
			require.NoError(t, err, "cluster-info ConfigMap should be created")

			// Parse the kubeconfig from the ConfigMap
			kubeconfigData := cm.Data[bootstrapapi.KubeConfigKey]
			parsedConfig, err := clientcmd.Load([]byte(kubeconfigData))
			require.NoError(t, err, "Should be able to parse kubeconfig from cluster-info")

			// Verify the server URL matches expectations
			require.Len(t, parsedConfig.Clusters, 1, "Should have exactly one cluster")

			var actualServerURL string
			for _, cluster := range parsedConfig.Clusters {
				actualServerURL = cluster.Server
				break
			}

			assert.Equal(t, tt.expectedServerURL, actualServerURL, tt.description)

			// Verify CA certificate is preserved
			for _, cluster := range parsedConfig.Clusters {
				assert.Equal(t, caCert, cluster.CertificateAuthorityData, "CA certificate should be preserved")
				break
			}
		})
	}
}

func TestBootstrapToken_EdgeCases(t *testing.T) {
	t.Run("handles missing kubeconfig clusters", func(t *testing.T) {
		client := fake.NewSimpleClientset()

		config := &Configuration{
			Kubeconfig: clientcmdapiv1.Config{
				Clusters: []clientcmdapiv1.NamedCluster{}, // Empty clusters
			},
			Parameters: Parameters{
				TenantControlPlanePublicAddress: "k8s-api.example.com",
				TenantControlPlanePort:          6443,
			},
		}

		// This should panic or error - the function assumes clusters[0] exists
		// In a real implementation, we might want to add validation
		assert.Panics(t, func() {
			BootstrapToken(client, config)
		}, "Should panic when no clusters are provided")
	})

	t.Run("preserves other kubeconfig properties", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		caCert := []byte("test-ca-certificate")

		config := &Configuration{
			Kubeconfig: clientcmdapiv1.Config{
				Clusters: []clientcmdapiv1.NamedCluster{
					{
						Cluster: clientcmdapiv1.Cluster{
							Server:                   "https://original.example.com:6443",
							CertificateAuthorityData: caCert,
							InsecureSkipTLSVerify:    false,
						},
					},
				},
			},
			Parameters: Parameters{
				TenantControlPlanePublicAddress: "k8s-api.example.com",
				TenantControlPlanePort:          6443,
			},
		}

		err := BootstrapToken(client, config)
		require.NoError(t, err)

		// Get and parse the cluster-info ConfigMap
		cm, err := client.CoreV1().ConfigMaps(metav1.NamespacePublic).Get(
			nil,
			bootstrapapi.ConfigMapClusterInfo,
			metav1.GetOptions{},
		)
		require.NoError(t, err)

		kubeconfigData := cm.Data[bootstrapapi.KubeConfigKey]
		parsedConfig, err := clientcmd.Load([]byte(kubeconfigData))
		require.NoError(t, err)

		// Verify CA certificate and other properties are preserved
		for _, cluster := range parsedConfig.Clusters {
			assert.Equal(t, caCert, cluster.CertificateAuthorityData)
			assert.Equal(t, "https://k8s-api.example.com:6443", cluster.Server)
			break
		}
	})
}
