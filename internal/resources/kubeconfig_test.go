// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

// newTestCA returns a self-signed CA certificate and private key in PEM format.
func newTestCA(t *testing.T) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating CA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kubernetes"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating CA certificate: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling CA key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// TestKubeconfigChecksumWithPregeneratedCertificates verifies that the kubeconfig checksum
// ignores the kubeadm configuration when certificates are pregenerated, so that a change to
// the control plane Service addresses does not regenerate the kubeconfig.
func TestKubeconfigChecksumWithPregeneratedCertificates(t *testing.T) {
	testCases := []struct {
		name         string
		pregenerated *v1alpha1.PreGeneratedCertificatesSpec
		expectStable bool
	}{
		{
			name:         "pregenerated certificates keep the kubeconfig checksum when the kubeadm config changes",
			pregenerated: &v1alpha1.PreGeneratedCertificatesSpec{},
			expectStable: true,
		},
		{
			name:         "generated certificates change the kubeconfig checksum when the kubeadm config changes",
			pregenerated: nil,
			expectStable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			caCert, caKey := newTestCA(t)

			tcp := &v1alpha1.TenantControlPlane{}
			tcp.SetName("tenant-01")
			tcp.SetNamespace("default")
			tcp.Spec.NetworkProfile.Port = 6443
			tcp.Spec.PreGeneratedCertificates = tc.pregenerated
			tcp.Status.Certificates.CA.SecretName = "tenant-01-ca"
			tcp.Status.KubeadmConfig.ConfigmapName = "tenant-01-kubeadm-config"

			config, err := kubeadm.CreateKubeadmInitConfiguration(kubeadm.Parameters{
				TenantControlPlaneName:      "tenant-01",
				TenantControlPlaneNamespace: "default",
				TenantControlPlaneAddress:   "10.0.0.1",
				TenantControlPlanePort:      6443,
				ETCDs:                       []string{"https://etcd.example.com:2379"},
				TenantControlPlaneVersion:   "v1.30.0",
			})
			if err != nil {
				t.Fatalf("creating kubeadm configuration: %v", err)
			}

			configData, err := kubeadm.GetKubeadmInitConfigurationMap(*config)
			if err != nil {
				t.Fatalf("encoding kubeadm configuration: %v", err)
			}

			// The advertise address changes when the control plane Service picks up its address.
			changed := *config
			changed.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = "10.0.0.5"

			changedData, err := kubeadm.GetKubeadmInitConfigurationMap(changed)
			if err != nil {
				t.Fatalf("encoding changed kubeadm configuration: %v", err)
			}

			if config.Checksum() == changed.Checksum() {
				t.Fatal("kubeadm configuration checksum must change for the test to be meaningful")
			}

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "tenant-01-kubeadm-config", Namespace: "default"},
				Data:       configData,
			}
			caSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "tenant-01-ca", Namespace: "default"},
				Data: map[string][]byte{
					kubeadmconstants.CACertName: caCert,
					kubeadmconstants.CAKeyName:  caKey,
				},
			}

			// The fake client does not assign UIDs; a kubeconfig secret without one is always
			// regenerated, so the existing secret is seeded with a UID as in a real cluster.
			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      (&KubeconfigResource{Name: "admin"}).getPrefixedName(tcp),
					Namespace: "default",
					UID:       "kubeconfig-uid",
				},
			}

			scheme := kubeadmUtilsTestScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap, caSecret, kubeconfigSecret).Build()

			reconcile := func() string {
				r := &KubeconfigResource{
					Client:                  cl,
					Name:                    "admin",
					KubeConfigFileName:      kubeadmconstants.AdminKubeConfigFileName,
					TmpDirectory:            t.TempDir(),
					CertExpirationThreshold: time.Hour,
				}

				if err = r.Define(ctx, tcp); err != nil {
					t.Fatalf("defining kubeconfig resource: %v", err)
				}

				if _, err = r.CreateOrUpdate(ctx, tcp); err != nil {
					t.Fatalf("reconciling kubeconfig resource: %v", err)
				}

				if err = r.UpdateTenantControlPlaneStatus(ctx, tcp); err != nil {
					t.Fatalf("updating kubeconfig status: %v", err)
				}

				return utilities.GetObjectChecksum(r.resource)
			}

			first := reconcile()

			if err = cl.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
				t.Fatalf("retrieving kubeadm configmap: %v", err)
			}

			configMap.Data = changedData
			if err = cl.Update(ctx, configMap); err != nil {
				t.Fatalf("updating kubeadm configmap: %v", err)
			}

			second := reconcile()

			if stable := first == second; stable != tc.expectStable {
				t.Fatalf("kubeconfig checksum stable = %t, want %t (first %q, second %q)", stable, tc.expectStable, first, second)
			}
		})
	}
}
