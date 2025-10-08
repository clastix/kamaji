// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

var _ = Describe("TCP CertSANs and PreGeneratedCertificates Mutual Exclusivity", func() {
	var tcp *kamajiv1alpha1.TenantControlPlane
	var certSANsHandler handlers.TenantControlPlaneCertSANs

	BeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "test-namespace",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{},
			},
		}
		certSANsHandler = handlers.TenantControlPlaneCertSANs{}
	})

	Context("when only certSANs is specified", func() {
		It("should pass validation", func() {
			tcp.Spec.NetworkProfile.CertSANs = []string{"api.example.com", "localhost"}

			err := certSANsHandler.ValidateCertSANs(tcp)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when only preGeneratedCertificates is specified", func() {
		It("should pass mutual exclusivity validation", func() {
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}

			// Test only the mutual exclusivity part, not the full validation
			err := certSANsHandler.ValidateCertSANs(tcp)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when both certSANs and preGeneratedCertificates are specified", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.CertSANs = []string{"api.example.com"}
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}
		})

		It("should fail validation from certSANs handler", func() {
			err := certSANsHandler.ValidateCertSANs(tcp)
			Expect(err).To(HaveOccurred())

			fieldErr, ok := err.(*field.Error)
			Expect(ok).To(BeTrue())
			Expect(fieldErr.Field).To(Equal("spec.networkProfile.certSANs"))
			Expect(fieldErr.BadValue).To(Equal(tcp.Spec.NetworkProfile.CertSANs))
			Expect(fieldErr.Detail).To(ContainSubstring("certSANs cannot be specified when preGeneratedCertificates is configured"))
		})

		It("should fail validation from preGeneratedCertificates handler", func() {
			// Test the mutual exclusivity check without requiring client connectivity
			// We can only test the simple mutual exclusivity check at the start of ValidatePreGeneratedCerts
			handler := handlers.TenantControlPlanePreGeneratedCerts{}
			err := handler.ValidatePreGeneratedCerts(context.Background(), tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preGeneratedCertificates cannot be specified when certSANs is configured"))
		})
	})

	Context("when neither certSANs nor preGeneratedCertificates are specified", func() {
		It("should pass validation from both handlers", func() {
			err := certSANsHandler.ValidateCertSANs(tcp)
			Expect(err).ToNot(HaveOccurred())

			// For preGen handler, we can only test the mutual exclusivity check
			handler := handlers.TenantControlPlanePreGeneratedCerts{}
			err = handler.ValidatePreGeneratedCerts(context.Background(), tcp)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when certSANs is empty and preGeneratedCertificates is specified", func() {
		It("should pass validation", func() {
			tcp.Spec.NetworkProfile.CertSANs = []string{} // explicitly empty
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}

			err := certSANsHandler.ValidateCertSANs(tcp)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("webhook admission response", func() {
		It("should return proper admission response on create with conflict", func() {
			tcp.Spec.NetworkProfile.CertSANs = []string{"api.example.com"}
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}

			response := certSANsHandler.OnCreate(tcp)
			_, err := response(context.Background(), admission.Request{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("certSANs cannot be specified when preGeneratedCertificates is configured"))
		})

		It("should return proper admission response on update with conflict", func() {
			tcp.Spec.NetworkProfile.CertSANs = []string{"api.example.com"}
			tcp.Spec.PreGeneratedCertificates = &kamajiv1alpha1.PreGeneratedCertificatesSpec{
				CA: &kamajiv1alpha1.CertificateReference{
					SecretName: "test-ca-cert",
				},
			}

			response := certSANsHandler.OnUpdate(tcp, tcp)
			_, err := response(context.Background(), admission.Request{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("certSANs cannot be specified when preGeneratedCertificates is configured"))
		})
	})
})
