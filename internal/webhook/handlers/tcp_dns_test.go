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

var _ = Describe("TCP DNS Validation Webhook", func() {
	var (
		ctx     context.Context
		handler handlers.TenantControlPlaneDNS
		tcp     *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()

		handler = handlers.TenantControlPlaneDNS{}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					ServiceCIDRs: []string{
						"10.96.0.0/16",
					},
					DNSServiceIPs: []string{
						"10.96.0.10",
					},
				},
			},
		}
	})

	Context("with valid configuration", func() {
		It("should allow creation", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow updates", func() {
			oldTCP := tcp.DeepCopy()

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when multiple DNS service IPs belong to the same service CIDR", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{"10.96.0.0/16", "fd00::/120"}

			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{"10.96.0.10", "10.96.0.11", "fd00::10"}
		})

		It("should allow creation", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when DNS service IP is outside all service CIDRs", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{
				"10.96.0.0/16",
			}

			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.97.0.10",
			}
		})

		It("should deny creation", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not contained in any configured service CIDR"))
		})
	})

	Context("with dual stack configuration", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{
				"10.96.0.0/16",
				"fd00::/120",
			}

			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.96.0.10",
				"fd00::10",
			}
		})

		It("should allow creation", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when using legacy serviceCidr", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = nil
			tcp.Spec.NetworkProfile.ServiceCIDR = "10.96.0.0/16"

			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.96.0.10",
			}
		})

		It("should allow creation", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("OnDelete operations", func() {
		It("should always allow delete operations", func() {
			admissionResponse := handler.OnDelete(tcp)

			_, err := admissionResponse(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when DNS service IP belongs to the second service CIDR", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{
				"10.96.0.0/16",
				"fd00::/120",
			}

			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.96.0.10",
				"fd00::10",
			}
		})

		It("should validate each DNS IP against all configured service CIDRs", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
