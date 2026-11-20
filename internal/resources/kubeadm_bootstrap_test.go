// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/yaml"

	"github.com/clastix/kamaji/internal/utilities"
)

func newClusterInfoTestKubeconfig(caData string) *clientcmdapiv1.Config {
	return &clientcmdapiv1.Config{
		Clusters: []clientcmdapiv1.NamedCluster{
			{
				Name: "",
				Cluster: clientcmdapiv1.Cluster{
					Server:                   "https://10.0.0.1:6443",
					CertificateAuthorityData: []byte(caData),
				},
			},
		},
	}
}

func newClusterInfoTestConfigMap(t *testing.T, caData string) corev1.ConfigMap {
	t.Helper()

	content, err := yaml.Marshal(newClusterInfoTestKubeconfig(caData))
	if err != nil {
		t.Fatal(err)
	}

	return corev1.ConfigMap{Data: map[string]string{bootstrapapi.KubeConfigKey: string(content)}}
}

// TestIsClusterInfoUpToDate prevents KubeadmBootstrap from skipping the cluster-info update
// during a CA rotation, which doesn't change the stored checksum.
func TestIsClusterInfoUpToDate(t *testing.T) {
	const (
		oldCA  = "old-ca"
		bundle = "new-ca\nold-ca"
	)

	clusterInfo := newClusterInfoTestConfigMap(t, oldCA)
	checksum := utilities.CalculateMapChecksum(clusterInfo.Data)

	for name, tc := range map[string]struct {
		clusterInfo    corev1.ConfigMap
		storedChecksum string
		kubeconfigCA   string
		expected       bool
	}{
		"checksum and CA unchanged": {
			clusterInfo:    clusterInfo,
			storedChecksum: checksum,
			kubeconfigCA:   oldCA,
			expected:       true,
		},
		"checksum unchanged, CA rotated": {
			clusterInfo:    clusterInfo,
			storedChecksum: checksum,
			kubeconfigCA:   bundle,
			expected:       false,
		},
		"checksum changed": {
			clusterInfo:    clusterInfo,
			storedChecksum: "outdated",
			kubeconfigCA:   oldCA,
			expected:       false,
		},
		"cluster-info missing": {
			clusterInfo:    corev1.ConfigMap{},
			storedChecksum: utilities.CalculateMapChecksum(map[string]string(nil)),
			kubeconfigCA:   oldCA,
			expected:       false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := isClusterInfoUpToDate(
				tc.clusterInfo,
				utilities.CalculateMapChecksum(tc.clusterInfo.Data),
				tc.storedChecksum,
				newClusterInfoTestKubeconfig(tc.kubeconfigCA),
			)
			if actual != tc.expected {
				t.Fatalf("expected %t, got %t", tc.expected, actual)
			}
		})
	}
}
