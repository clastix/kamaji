// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

var _ = Describe("FrontProxyCACertificate Resource", func() {
	var (
		ctx              context.Context
		frontProxyCACert resources.FrontProxyCACertificate
		tcp              *kamajiv1alpha1.TenantControlPlane
		secret           *corev1.Secret
		testCert         []byte
		testKey          []byte
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Register the custom scheme for TenantControlPlane CRD
		err := kamajiv1alpha1.AddToScheme(scheme.Scheme)
		Expect(err).ToNot(HaveOccurred())

		// Generate test certificate and key
		testCert, testKey, err = generateTestFrontProxyCACertificate()
		Expect(err).ToNot(HaveOccurred())

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "test-namespace",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: func() *int32 { i := int32(1); return &i }(),
					},
				},
				DataStore: "test-datastore",
			},
		}
	})

	Context("when using pregenerated certificates", func() {
		Context("with kubeadm format keys", func() {
			BeforeEach(func() {
				// Create secret with kubeadm format keys
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "front-proxy-ca-secret",
						Namespace: tcp.Namespace,
					},
					Data: map[string][]byte{
						kubeadmconstants.FrontProxyCACertName: testCert,
						kubeadmconstants.FrontProxyCAKeyName:  testKey,
					},
				}

				tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
					FrontProxyCA: &kamajiv1alpha1.CertificateReference{
						SecretName: secret.Name,
					},
				}

				client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()
				frontProxyCACert = resources.FrontProxyCACertificate{
					Client:       client,
					TmpDirectory: "/tmp",
				}

				err := frontProxyCACert.Define(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should successfully use kubeadm format keys", func() {
				result, err := frontProxyCACert.CreateOrUpdate(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				// Verify the operation completed without error
				// In a full integration test, we would verify the actual resource creation
				// but this requires more complex setup
			})
		})

		Context("with TLS format keys", func() {
			BeforeEach(func() {
				// Create secret with TLS format keys
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "front-proxy-ca-tls-secret",
						Namespace: tcp.Namespace,
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey:       testCert,
						corev1.TLSPrivateKeyKey: testKey,
					},
				}

				tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
					FrontProxyCA: &kamajiv1alpha1.CertificateReference{
						SecretName: secret.Name,
					},
				}

				client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()
				frontProxyCACert = resources.FrontProxyCACertificate{
					Client:       client,
					TmpDirectory: "/tmp",
				}

				err := frontProxyCACert.Define(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should successfully fallback to TLS format keys", func() {
				result, err := frontProxyCACert.CreateOrUpdate(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				// Verify the operation completed without error
				// The fallback logic is tested in the actual implementation
			})
		})

		Context("with explicit certificate and key names", func() {
			BeforeEach(func() {
				// Create secret with custom key names
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "front-proxy-ca-custom-secret",
						Namespace: tcp.Namespace,
					},
					Data: map[string][]byte{
						"custom.crt": testCert,
						"custom.key": testKey,
					},
				}

				tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
					FrontProxyCA: &kamajiv1alpha1.CertificateReference{
						SecretName:     secret.Name,
						CertificateKey: "custom.crt",
						PrivateKeyKey:  "custom.key",
					},
				}

				client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()
				frontProxyCACert = resources.FrontProxyCACertificate{
					Client:       client,
					TmpDirectory: "/tmp",
				}

				err := frontProxyCACert.Define(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use explicitly specified key names", func() {
				result, err := frontProxyCACert.CreateOrUpdate(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				// Verify the operation completed without error
				// Custom key names would be handled by the implementation
			})
		})

		Context("with missing keys", func() {
			BeforeEach(func() {
				// Create secret without required keys
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "front-proxy-ca-missing-secret",
						Namespace: tcp.Namespace,
					},
					Data: map[string][]byte{
						"other-key": []byte("other-data"),
					},
				}

				tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
					FrontProxyCA: &kamajiv1alpha1.CertificateReference{
						SecretName: secret.Name,
					},
				}

				client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()
				frontProxyCACert = resources.FrontProxyCACertificate{
					Client:       client,
					TmpDirectory: "/tmp",
				}

				err := frontProxyCACert.Define(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error when neither kubeadm nor TLS format keys are found", func() {
				_, err := frontProxyCACert.CreateOrUpdate(ctx, tcp)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found in secret"))
				Expect(err.Error()).To(ContainSubstring("fallback key"))
				Expect(err.Error()).To(ContainSubstring("also not found"))
			})
		})

		Context("with partial keys", func() {
			BeforeEach(func() {
				// Create secret with only certificate, missing private key
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "front-proxy-ca-partial-secret",
						Namespace: tcp.Namespace,
					},
					Data: map[string][]byte{
						kubeadmconstants.FrontProxyCACertName: testCert,
						// Missing private key
					},
				}

				tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
					FrontProxyCA: &kamajiv1alpha1.CertificateReference{
						SecretName: secret.Name,
					},
				}

				client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()
				frontProxyCACert = resources.FrontProxyCACertificate{
					Client:       client,
					TmpDirectory: "/tmp",
				}

				err := frontProxyCACert.Define(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error when private key is missing", func() {
				_, err := frontProxyCACert.CreateOrUpdate(ctx, tcp)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("private key"))
				Expect(err.Error()).To(ContainSubstring("not found in secret"))
			})
		})

		Context("with mixed format fallback", func() {
			BeforeEach(func() {
				// Create secret with kubeadm certificate but TLS private key
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "front-proxy-ca-mixed-secret",
						Namespace: tcp.Namespace,
					},
					Data: map[string][]byte{
						kubeadmconstants.FrontProxyCACertName: testCert,
						corev1.TLSPrivateKeyKey:               testKey,
					},
				}

				tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
					FrontProxyCA: &kamajiv1alpha1.CertificateReference{
						SecretName: secret.Name,
					},
				}

				client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()
				frontProxyCACert = resources.FrontProxyCACertificate{
					Client:       client,
					TmpDirectory: "/tmp",
				}

				err := frontProxyCACert.Define(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle mixed format keys correctly", func() {
				result, err := frontProxyCACert.CreateOrUpdate(ctx, tcp)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				// Verify both certificate and key are found and used
				// Mixed format handling is tested in the implementation
			})
		})
	})
})

func generateTestFrontProxyCACertificate() ([]byte, []byte, error) {
	// Generate a test private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create a certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Organization"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    "test-front-proxy-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	// Generate the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, keyPEM, nil
}
