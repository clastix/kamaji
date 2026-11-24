// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
)

// TestClusterAdminRBACPhaseRemovesTemporaryDirectory verifies that the
// cluster-admin RBAC phase leaves nothing behind in the temporary directory.
// The phase has no checksum, so it runs at every reconciliation, and it writes
// the admin and super-admin kubeconfig files there: anything left over piles
// up for the lifetime of the pod.
func TestClusterAdminRBACPhaseRemovesTemporaryDirectory(t *testing.T) {
	pkiDir := filepath.Join(t.TempDir(), "pki")
	workDir := t.TempDir()
	// os.MkdirTemp("", ...) resolves the base directory through TMPDIR.
	t.Setenv("TMPDIR", workDir)

	tcp := &v1alpha1.TenantControlPlane{}
	tcp.SetName("test-tcp")
	tcp.SetNamespace("default")
	tcp.SetUID("7c0f8a52-6a6c-4b55-9f0e-3f6f2d1c9b11")
	tcp.Spec.NetworkProfile.Port = 6443
	tcp.Status.Certificates.CA.SecretName = "test-tcp-ca"

	config, err := kubeadm.CreateKubeadmInitConfiguration(kubeadm.Parameters{
		TenantControlPlaneName:          tcp.GetName(),
		TenantControlPlaneNamespace:     tcp.GetNamespace(),
		TenantControlPlaneEndpoint:      "1.2.3.4:6443",
		TenantControlPlaneAddress:       "1.2.3.4",
		TenantControlPlanePort:          6443,
		TenantControlPlaneClusterDomain: "cluster.local",
		TenantControlPlanePodCIDR:       []string{"10.244.0.0/16"},
		TenantControlPlaneServiceCIDR:   []string{"10.96.0.0/12"},
		TenantControlPlaneVersion:       "v1.34.0",
		ETCDs:                           []string{"https://etcd.example:2379"},
	})
	if err != nil {
		t.Fatalf("creating kubeadm configuration: %v", err)
	}

	config.InitConfiguration.CertificatesDir = pkiDir

	ca, err := kubeadm.GenerateCACertificatePrivateKeyPair(kubeadmconstants.CACertAndKeyBaseName, config)
	if err != nil {
		t.Fatalf("generating CA: %v", err)
	}

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcp.Status.Certificates.CA.SecretName,
			Namespace: tcp.GetNamespace(),
		},
		Data: map[string][]byte{
			kubeadmconstants.CACertName: ca.Certificate,
			kubeadmconstants.CAKeyName:  ca.PrivateKey,
		},
	}

	phase := &KubeadmPhase{
		Client: fake.NewClientBuilder().WithScheme(kubeadmUtilsTestScheme(t)).WithObjects(caSecret).Build(),
		Phase:  PhaseClusterAdminRBAC,
	}

	fn, err := phase.GetKubeadmFunction(t.Context(), tcp)
	if err != nil {
		t.Fatalf("retrieving kubeadm function: %v", err)
	}

	if _, err = fn(k8sfake.NewClientset(), config); err != nil {
		t.Fatalf("running the cluster-admin RBAC phase: %v", err)
	}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatalf("reading temporary directory: %v", err)
	}

	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		t.Fatalf("expected the phase to remove its temporary directory, found %v", names)
	}
}
