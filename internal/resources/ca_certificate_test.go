// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/clastix/kamaji/api/v1alpha1"
)

func newCACertificateTestPair(t *testing.T, commonName string) (certificate, privateKey []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func concatenate(contents ...[]byte) []byte {
	return bytes.Join(contents, nil)
}

// TestCACertificateMutate covers the CA Secret handling during an overlapping CA rotation,
// where ca.crt is set by hand to a bundle of the new and old CA.
func TestCACertificateMutate(t *testing.T) {
	newCA, newKey := newCACertificateTestPair(t, "new")
	oldCA, oldKey := newCACertificateTestPair(t, "old")

	for name, tc := range map[string]struct {
		data map[string][]byte
		// expected Secret content after the mutation, nil when it must be left untouched
		expected      map[string][]byte
		expectErr     bool
		regenerateErr bool
	}{
		"single CA is left untouched": {
			data: map[string][]byte{
				kubeadmconstants.CACertName: oldCA, kubeadmconstants.CAKeyName: oldKey,
				corev1.TLSCertKey: oldCA, corev1.TLSPrivateKeyKey: oldKey,
			},
		},
		"single CA gets the Cluster API keys": {
			data: map[string][]byte{kubeadmconstants.CACertName: oldCA, kubeadmconstants.CAKeyName: oldKey},
			expected: map[string][]byte{
				kubeadmconstants.CACertName: oldCA, kubeadmconstants.CAKeyName: oldKey,
				corev1.TLSCertKey: oldCA, corev1.TLSPrivateKeyKey: oldKey,
			},
		},
		"bundle signs with its first certificate": {
			data: map[string][]byte{
				kubeadmconstants.CACertName: concatenate(newCA, oldCA), kubeadmconstants.CAKeyName: newKey,
				corev1.TLSCertKey: oldCA, corev1.TLSPrivateKeyKey: oldKey,
			},
			expected: map[string][]byte{
				kubeadmconstants.CACertName: concatenate(newCA, oldCA), kubeadmconstants.CAKeyName: newKey,
				corev1.TLSCertKey: newCA, corev1.TLSPrivateKeyKey: newKey,
			},
		},
		"bundle with the old CA first is not regenerated": {
			data: map[string][]byte{
				kubeadmconstants.CACertName: concatenate(oldCA, newCA), kubeadmconstants.CAKeyName: newKey,
				corev1.TLSCertKey: oldCA, corev1.TLSPrivateKeyKey: oldKey,
			},
			expectErr: true,
		},
		"bundle with a malformed certificate is rejected": {
			data: map[string][]byte{
				kubeadmconstants.CACertName: concatenate(newCA, []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n")),
				kubeadmconstants.CAKeyName:  newKey,
				corev1.TLSCertKey:           newCA, corev1.TLSPrivateKeyKey: newKey,
			},
			expectErr: true,
		},
		"invalid single CA is regenerated": {
			data: map[string][]byte{
				kubeadmconstants.CACertName: oldCA, kubeadmconstants.CAKeyName: newKey,
				corev1.TLSCertKey: oldCA, corev1.TLSPrivateKeyKey: newKey,
			},
			regenerateErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tcp := &v1alpha1.TenantControlPlane{}
			tcp.SetName("tenant")
			tcp.SetNamespace("default")
			tcp.SetUID(types.UID("tenant-uid"))

			original := make(map[string][]byte, len(tc.data))
			for key, value := range tc.data {
				original[key] = value
			}

			secret := &corev1.Secret{Data: tc.data}
			secret.SetName("tenant-ca")
			secret.SetNamespace("default")
			secret.SetUID(types.UID("secret-uid"))

			r := &CACertificate{
				resource: secret,
				Client:   fake.NewClientBuilder().WithScheme(kubeadmUtilsTestScheme(t)).Build(),
			}

			err := r.mutate(context.Background(), tcp)()

			switch {
			case tc.regenerateErr:
				// the fallback generates a new CA from the stored kubeadm configuration, missing here
				if !k8serrors.IsNotFound(err) {
					t.Fatalf("expected the CA to be regenerated, got %v", err)
				}
			case tc.expectErr:
				if err == nil || k8serrors.IsNotFound(err) {
					t.Fatalf("expected the CA Secret to be rejected without regeneration, got %v", err)
				}
			case err != nil:
				t.Fatal(err)
			}

			expected := tc.expected
			if expected == nil {
				expected = original
			}

			if len(secret.Data) != len(expected) {
				t.Fatalf("expected keys %d, got %d", len(expected), len(secret.Data))
			}

			for key, value := range expected {
				if !bytes.Equal(secret.Data[key], value) {
					t.Fatalf("unexpected %s content", key)
				}
			}
		})
	}
}
