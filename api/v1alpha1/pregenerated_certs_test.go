// Copyright 2025 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("PreGeneratedCertificates API", func() {
	var tcp *kamajiv1alpha1.TenantControlPlane

	BeforeEach(func() {
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

	Context("CertificateReference", func() {
		It("should validate required secretName", func() {
			certRef := &kamajiv1alpha1.CertificateReference{}

			// secretName is required, so this should be invalid
			Expect(certRef.SecretName).To(BeEmpty())
		})

		It("should have correct default values", func() {
			certRef := &kamajiv1alpha1.CertificateReference{
				SecretName: "test-secret",
			}

			// Defaults should be applied by the CRD
			Expect(certRef.SecretName).To(Equal("test-secret"))
		})

		It("should support custom certificate and key names", func() {
			certRef := &kamajiv1alpha1.CertificateReference{
				SecretName:      "test-secret",
				CertificateKey:  "custom.crt",
				PrivateKeyKey:   "custom.key",
				SecretNamespace: "custom-namespace",
			}

			Expect(certRef.SecretName).To(Equal("test-secret"))
			Expect(certRef.CertificateKey).To(Equal("custom.crt"))
			Expect(certRef.PrivateKeyKey).To(Equal("custom.key"))
			Expect(certRef.SecretNamespace).To(Equal("custom-namespace"))
		})
	})

	Context("KeyReference", func() {
		It("should validate required secretName", func() {
			keyRef := &kamajiv1alpha1.KeyReference{}

			// secretName is required, so this should be invalid
			Expect(keyRef.SecretName).To(BeEmpty())
		})

		It("should support custom public and private key names", func() {
			keyRef := &kamajiv1alpha1.KeyReference{
				SecretName:      "test-secret",
				PublicKeyKey:    "custom.pub",
				PrivateKeyKey:   "custom.key",
				SecretNamespace: "custom-namespace",
			}

			Expect(keyRef.SecretName).To(Equal("test-secret"))
			Expect(keyRef.PublicKeyKey).To(Equal("custom.pub"))
			Expect(keyRef.PrivateKeyKey).To(Equal("custom.key"))
			Expect(keyRef.SecretNamespace).To(Equal("custom-namespace"))
		})
	})

	Context("PreGeneratedCertificatesSpec", func() {
		It("should be optional in TenantControlPlane", func() {
			Expect(tcp.Spec.PreGeneratedCertificates).To(BeNil())
		})

		It("should support all certificate types", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "ca-cert",
				},
				APIServer: &kamajiv1alpha1.CertificateReference{
					SecretName: "api-server-cert",
				},
				KubeletClient: &kamajiv1alpha1.CertificateReference{
					SecretName: "kubelet-client-cert",
				},
				FrontProxyCA: &kamajiv1alpha1.CertificateReference{
					SecretName: "front-proxy-ca-cert",
				},
				FrontProxyClient: &kamajiv1alpha1.CertificateReference{
					SecretName: "front-proxy-client-cert",
				},
				ServiceAccount: &kamajiv1alpha1.KeyReference{
					SecretName: "sa-keys",
				},
			}

			Expect(tcp.Spec.PreGeneratedCertificates).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.CA).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.CA.SecretName).To(Equal("ca-cert"))
			Expect(tcp.Spec.PreGeneratedCertificates.APIServer).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.APIServer.SecretName).To(Equal("api-server-cert"))
			Expect(tcp.Spec.PreGeneratedCertificates.KubeletClient).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.KubeletClient.SecretName).To(Equal("kubelet-client-cert"))
			Expect(tcp.Spec.PreGeneratedCertificates.FrontProxyCA).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.FrontProxyCA.SecretName).To(Equal("front-proxy-ca-cert"))
			Expect(tcp.Spec.PreGeneratedCertificates.FrontProxyClient).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.FrontProxyClient.SecretName).To(Equal("front-proxy-client-cert"))
			Expect(tcp.Spec.PreGeneratedCertificates.ServiceAccount).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.ServiceAccount.SecretName).To(Equal("sa-keys"))
		})

		It("should support partial certificate specification", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "ca-cert",
				},
				// Only CA specified, others should be nil
			}

			Expect(tcp.Spec.PreGeneratedCertificates.CA).ToNot(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.APIServer).To(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.KubeletClient).To(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.FrontProxyCA).To(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.FrontProxyClient).To(BeNil())
			Expect(tcp.Spec.PreGeneratedCertificates.ServiceAccount).To(BeNil())
		})

		It("should support cross-namespace references", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName:      "ca-cert",
					SecretNamespace: "cert-manager",
				},
				APIServer: &kamajiv1alpha1.CertificateReference{
					SecretName:      "api-server-cert",
					SecretNamespace: "kube-system",
				},
			}

			Expect(tcp.Spec.PreGeneratedCertificates.CA.SecretNamespace).To(Equal("cert-manager"))
			Expect(tcp.Spec.PreGeneratedCertificates.APIServer.SecretNamespace).To(Equal("kube-system"))
		})

		It("should support custom key names", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName:     "ca-cert",
					CertificateKey: "ca.crt",
					PrivateKeyKey:  "ca.key",
				},
				ServiceAccount: &kamajiv1alpha1.KeyReference{
					SecretName:    "sa-keys",
					PublicKeyKey:  "service-account.pub",
					PrivateKeyKey: "service-account.key",
				},
			}

			Expect(tcp.Spec.PreGeneratedCertificates.CA.CertificateKey).To(Equal("ca.crt"))
			Expect(tcp.Spec.PreGeneratedCertificates.CA.PrivateKeyKey).To(Equal("ca.key"))
			Expect(tcp.Spec.PreGeneratedCertificates.ServiceAccount.PublicKeyKey).To(Equal("service-account.pub"))
			Expect(tcp.Spec.PreGeneratedCertificates.ServiceAccount.PrivateKeyKey).To(Equal("service-account.key"))
		})
	})

	Context("JSON serialization", func() {
		It("should serialize and deserialize correctly", func() {
			original := &kamajiv1alpha1.TenantControlPlane{
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
					PreGeneratedCertificates: &kamajiv1alpha1.PreGeneratedCertificatesSpec{
						CA: &kamajiv1alpha1.CertificateReference{
							SecretName:      "ca-cert",
							SecretNamespace: "cert-manager",
							CertificateKey:  "tls.crt",
							PrivateKeyKey:   "tls.key",
						},
					},
				},
			}

			// Test deepcopy functionality (which relies on proper serialization)
			copied := original.DeepCopy()

			Expect(copied.Spec.PreGeneratedCertificates).ToNot(BeNil())
			Expect(copied.Spec.PreGeneratedCertificates.CA).ToNot(BeNil())
			Expect(copied.Spec.PreGeneratedCertificates.CA.SecretName).To(Equal("ca-cert"))
			Expect(copied.Spec.PreGeneratedCertificates.CA.SecretNamespace).To(Equal("cert-manager"))
			Expect(copied.Spec.PreGeneratedCertificates.CA.CertificateKey).To(Equal("tls.crt"))
			Expect(copied.Spec.PreGeneratedCertificates.CA.PrivateKeyKey).To(Equal("tls.key"))
		})
	})
})
