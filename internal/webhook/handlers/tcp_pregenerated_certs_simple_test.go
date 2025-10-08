// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

var _ = Describe("TCP PreGenerated Certificates Simple Tests", func() {
	var (
		ctx     context.Context
		handler handlers.TenantControlPlanePreGeneratedCerts
		tcp     *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background() //nolint:fatcontext
		handler = handlers.TenantControlPlanePreGeneratedCerts{
			Client: k8sClient,
		}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{},
		}
	})

	Context("basic functionality", func() {
		It("should allow creation when no pregenerated certificates are specified", func() {
			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ops).To(BeNil())
		})

		It("should reject creation when pregenerated certificates conflict with certSANs", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName:     "test-ca",
					CertificateKey: "ca.crt",
				},
			}
			tcp.Spec.NetworkProfile.CertSANs = []string{"example.com"}

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())
		})

		It("should allow update when pregenerated certificates conflict with certSANs", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName:     "test-ca",
					CertificateKey: "ca.crt",
				},
			}
			tcp.Spec.NetworkProfile.CertSANs = []string{"example.com"}

			oldTcp := tcp.DeepCopy()
			ops, err := handler.OnUpdate(tcp, oldTcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())
		})
	})

	Context("certificate type coverage validation", func() {
		It("should reject any certificate type when certSANs is configured", func() {
			By("Testing with APIServer certificate")
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				APIServer: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-apiserver",
				},
			}
			tcp.Spec.NetworkProfile.CertSANs = []string{"example.com"}

			ops, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())

			By("Testing with KubeletClient certificate")
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				KubeletClient: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-kubelet-client",
				},
			}

			ops, err = handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())

			By("Testing with ServiceAccount key")
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				ServiceAccount: &kamajiv1alpha1.KeyReference{
					SecretName: "test-service-account",
				},
			}

			ops, err = handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())

			By("Testing with FrontProxyCA certificate")
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				FrontProxyCA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-front-proxy-ca",
				},
			}

			ops, err = handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())

			By("Testing with FrontProxyClient certificate")
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				FrontProxyClient: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-front-proxy-client",
				},
			}

			ops, err = handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
			Expect(ops).To(BeNil())
		})
	})
})
