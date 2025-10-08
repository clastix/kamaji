// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

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
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

var _ = Describe("TCP PreGenerated Certs Webhook", func() {
	var (
		ctx         context.Context
		handler     handlers.TenantControlPlanePreGeneratedCerts
		tcp         *kamajiv1alpha1.TenantControlPlane
		validSecret *corev1.Secret
		caCert      []byte
		caKey       []byte
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Generate a valid CA certificate and key for testing
		var err error
		caCert, caKey, err = generateCACertificate()
		Expect(err).ToNot(HaveOccurred())

		validSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ca-cert",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				corev1.TLSCertKey:       caCert,
				corev1.TLSPrivateKeyKey: caKey,
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithObjects(validSecret).
			Build()

		handler = handlers.TenantControlPlanePreGeneratedCerts{
			Client: fakeClient,
		}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "test-namespace",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: kamajiv1alpha1.ServiceTypeClusterIP,
					},
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: "v1.28.0",
				},
			},
		}
	})

	Context("when PreGeneratedCertificates is nil", func() {
		It("should pass validation", func() {
			tcp.Spec.PreGeneratedCertificates = nil

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})
	})

	Context("when valid PreGeneratedCertificates are provided", func() {
		BeforeEach(func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}
		})

		It("should pass validation with valid CA certificate", func() {
			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})

		It("should pass validation with explicit certificate and key fields", func() {
			tcp.Spec.PreGeneratedCertificates.CA.CertificateKey = corev1.TLSCertKey
			tcp.Spec.PreGeneratedCertificates.CA.PrivateKeyKey = corev1.TLSPrivateKeyKey

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})

		It("should pass validation with cross-namespace secret reference", func() {
			crossNsSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-cert",
					Namespace: "cert-manager",
				},
				Data: map[string][]byte{
					corev1.TLSCertKey:       caCert,
					corev1.TLSPrivateKeyKey: caKey,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithObjects(validSecret, crossNsSecret).
				Build()
			handler.Client = fakeClient

			tcp.Spec.PreGeneratedCertificates.CA.SecretName = "cross-ns-cert"
			tcp.Spec.PreGeneratedCertificates.CA.SecretNamespace = "cert-manager"

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})
	})

	Context("when invalid PreGeneratedCertificates are provided", func() {
		It("should fail when secret does not exist", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "non-existent-secret",
				},
			}

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get secret"))
			Expect(ops).To(BeNil())
		})

		It("should fail when certificate key is missing from secret", func() {
			invalidSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cert",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					corev1.TLSPrivateKeyKey: caKey, // Only private key, no certificate
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithObjects(validSecret, invalidSecret).
				Build()
			handler.Client = fakeClient

			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "invalid-cert",
				},
			}

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("certificate key"))
			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(ops).To(BeNil())
		})

		It("should fail when private key is missing from secret", func() {
			invalidSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cert",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					corev1.TLSCertKey: caCert, // Only certificate, no private key
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithObjects(validSecret, invalidSecret).
				Build()
			handler.Client = fakeClient

			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "invalid-cert",
				},
			}

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("private key"))
			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(ops).To(BeNil())
		})

		It("should fail when certificate and private key don't match", func() {
			// Generate another key pair that doesn't match
			otherCert, _, err := generateCACertificate()
			Expect(err).ToNot(HaveOccurred())

			mismatchedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mismatched-cert",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					corev1.TLSCertKey:       otherCert, // Different certificate
					corev1.TLSPrivateKeyKey: caKey,     // Original private key
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithObjects(validSecret, mismatchedSecret).
				Build()
			handler.Client = fakeClient

			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "mismatched-cert",
				},
			}

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("certificate-private key pair is not valid"))
			Expect(ops).To(BeNil())
		})
	})

	Context("when validating Service Account keys", func() {
		BeforeEach(func() {
			// Generate RSA key pair for service account
			saPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			saPrivKeyBytes := pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(saPrivKey),
			})

			saPubKeyBytes, err := x509.MarshalPKIXPublicKey(&saPrivKey.PublicKey)
			Expect(err).ToNot(HaveOccurred())

			saPubKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: saPubKeyBytes,
			})

			saSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sa-keys",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					kubeadmconstants.ServiceAccountPublicKeyName:  saPubKeyPEM,
					kubeadmconstants.ServiceAccountPrivateKeyName: saPrivKeyBytes,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithObjects(validSecret, saSecret).
				Build()
			handler.Client = fakeClient

			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				ServiceAccount: &kamajiv1alpha1.KeyReference{
					SecretName: "sa-keys",
				},
			}
		})

		It("should pass validation with valid service account keys", func() {
			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})
	})

	Context("OnUpdate validation", func() {
		It("should validate updated TenantControlPlane with pregenerated certs", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}

			oldTCP := tcp.DeepCopy()

			ops, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})
	})

	Context("OnDelete validation", func() {
		It("should always pass validation on delete", func() {
			ops, err := handler.OnDelete(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeNil())
		})
	})
})

// generateCACertificate generates a valid CA certificate and private key for testing.
func generateCACertificate() ([]byte, []byte, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, keyPEM, nil
}
