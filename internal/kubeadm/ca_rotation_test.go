// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"bytes"
	"crypto/x509"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/clastix/kamaji/internal/crypto"
)

func newTestConfiguration(t *testing.T) *Configuration {
	t.Helper()

	config, err := CreateKubeadmInitConfiguration(Parameters{
		TenantControlPlaneName:        "tenant",
		TenantControlPlaneNamespace:   "default",
		TenantControlPlaneEndpoint:    "10.0.0.1:6443",
		TenantControlPlaneAddress:     "10.0.0.1",
		TenantControlPlanePort:        6443,
		TenantControlPlaneServiceCIDR: []string{"10.96.0.0/16"},
		TenantControlPlaneVersion:     "v1.34.2",
		ETCDs:                         []string{"http://127.0.0.1:2379"},
	})
	if err != nil {
		t.Fatal(err)
	}

	config.InitConfiguration.CertificatesDir = filepath.Join(t.TempDir(), "pki")

	return config
}

func newTestCA(t *testing.T) *CertificatePrivateKeyPair {
	t.Helper()

	ca, err := GenerateCACertificatePrivateKeyPair(kubeadmconstants.CACertAndKeyBaseName, newTestConfiguration(t))
	if err != nil {
		t.Fatal(err)
	}

	return ca
}

func newTestServingCertificate(t *testing.T, ca CertificatePrivateKeyPair) []byte {
	t.Helper()

	leaf, err := GenerateCertificatePrivateKeyPair(kubeadmconstants.APIServerCertAndKeyBaseName, newTestConfiguration(t), ca)
	if err != nil {
		t.Fatal(err)
	}

	return leaf.Certificate
}

func TestCertificateAuthorityRotationPhases(t *testing.T) {
	oldCA, newCA := newTestCA(t), newTestCA(t)
	bundle := append(append([]byte{}, newCA.Certificate...), oldCA.Certificate...)

	oldLeaf := newTestServingCertificate(t, *oldCA)
	// Phase 1: new CA is the signer, old leaves must not be reissued.
	if ok, err := crypto.VerifyCertificate(oldLeaf, bundle, x509.ExtKeyUsageServerAuth); !ok {
		t.Fatalf("leaf signed by old CA must be valid against the bundle: %v", err)
	}
	// Phase 3: old CA is dropped, old leaves must be reissued.
	if ok, _ := crypto.VerifyCertificate(oldLeaf, newCA.Certificate, x509.ExtKeyUsageServerAuth); ok {
		t.Fatal("leaf signed by old CA must be invalid against the new CA only")
	}
	// Leaves generated from the bundle are signed by the first CA.
	newLeaf := newTestServingCertificate(t, CertificatePrivateKeyPair{Certificate: bundle, PrivateKey: newCA.PrivateKey})
	if ok, err := crypto.VerifyCertificate(newLeaf, newCA.Certificate, x509.ExtKeyUsageServerAuth); !ok {
		t.Fatalf("leaf generated from the bundle must be signed by the new CA: %v", err)
	}

	for name, ca := range map[string]CertificatePrivateKeyPair{
		"bundle":    {Certificate: bundle, PrivateKey: newCA.PrivateKey},
		"single CA": *newCA,
	} {
		t.Run(name, func(t *testing.T) {
			content, err := CreateKubeconfig(kubeadmconstants.AdminKubeConfigFileName, ca, newTestConfiguration(t))
			if err != nil {
				t.Fatal(err)
			}
			// Prevents the endless regeneration of kubeconfigs by Kamaji.
			if !IsKubeconfigCAValid(content, ca.Certificate) {
				t.Fatal("kubeconfig CA data must match the CA secret content")
			}

			kubeconfig, err := clientcmd.Load(content)
			if err != nil {
				t.Fatal(err)
			}

			for _, authInfo := range kubeconfig.AuthInfos {
				if ok, verifyErr := crypto.VerifyCertificate(authInfo.ClientCertificateData, newCA.Certificate, x509.ExtKeyUsageClientAuth); !ok {
					t.Fatalf("client certificate must be signed by the new CA: %v", verifyErr)
				}
			}
		})
	}
}

func TestWithCertificateAuthorityBundleKeepsSingleCAUntouched(t *testing.T) {
	ca := newTestCA(t)

	content, err := CreateKubeconfig(kubeadmconstants.AdminKubeConfigFileName, *ca, newTestConfiguration(t))
	if err != nil {
		t.Fatal(err)
	}

	unchanged, err := withCertificateAuthorityBundle(content, ca.Certificate)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, unchanged) {
		t.Fatal("single CA kubeconfig must be byte-identical to the upstream one")
	}
}

func TestWithCertificateAuthorityBundleRejectsMalformedBundle(t *testing.T) {
	ca := newTestCA(t)

	content, err := CreateKubeconfig(kubeadmconstants.AdminKubeConfigFileName, *ca, newTestConfiguration(t))
	if err != nil {
		t.Fatal(err)
	}

	malformed := append(append([]byte{}, ca.Certificate...), "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"...)
	if _, err = withCertificateAuthorityBundle(content, malformed); err == nil {
		t.Fatal("a malformed CA bundle must be reported, not replaced by the first certificate")
	}
}
