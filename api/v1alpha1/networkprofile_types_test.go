// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NetworkProfile validation", func() {
	var (
		ctx context.Context
		tcp *TenantControlPlane
	)

	const (
		ipv6CIDRBlock = "fd00::/108"
	)

	BeforeEach(func() {
		ctx = context.Background()
		tcp = &TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "tcp-network-",
				Namespace:    "default",
			},
			Spec: TenantControlPlaneSpec{
				ControlPlane: ControlPlane{
					Service: ServiceSpec{
						ServiceType: ServiceTypeClusterIP,
					},
				},
			},
		}
	})

	AfterEach(func() {
		// When creation is denied by validation, GenerateName is never resolved
		// and tcp.Name remains empty, so there is nothing to delete.
		if tcp.Name == "" {
			return
		}
		if err := k8sClient.Delete(ctx, tcp); err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("serviceCidr", func() {
		It("allows creation with the default IPv4 CIDR", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = "10.96.0.0/16"

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with a non-default valid IPv4 CIDR", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = "172.16.0.0/12"

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with a valid IPv6 CIDR", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = ipv6CIDRBlock

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation when serviceCidr is empty", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = ""

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("denies creation with a plain IP address instead of a CIDR", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = "10.96.0.1"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("serviceCidr must be empty or a valid CIDR"))
		})

		It("denies creation with an arbitrary non-CIDR string", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = "not-a-cidr"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("serviceCidr must be empty or a valid CIDR"))
		})
	})

	Context("podCidr", func() {
		It("allows creation with the default IPv4 CIDR", func() {
			tcp.Spec.NetworkProfile.PodCIDR = "10.244.0.0/16"

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with a non-default valid IPv4 CIDR", func() {
			tcp.Spec.NetworkProfile.PodCIDR = "192.168.128.0/17"

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with a valid IPv6 CIDR", func() {
			tcp.Spec.NetworkProfile.PodCIDR = "2001:db8::/48"

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation when podCidr is empty", func() {
			tcp.Spec.NetworkProfile.PodCIDR = ""

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("denies creation with a plain IP address instead of a CIDR", func() {
			tcp.Spec.NetworkProfile.PodCIDR = "10.244.0.1"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("podCidr must be empty or a valid CIDR"))
		})

		It("denies creation with an arbitrary non-CIDR string", func() {
			tcp.Spec.NetworkProfile.PodCIDR = "not-a-cidr"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("podCidr must be empty or a valid CIDR"))
		})
	})

	Context("loadBalancerSourceRanges CIDR format", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeLoadBalancer
		})

		It("allows creation with a single valid CIDR", func() {
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"10.0.0.0/8"}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with multiple valid CIDRs", func() {
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{
				"10.0.0.0/8",
				"192.168.0.0/24",
				"172.16.0.0/12",
			}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with valid IPv6 CIDRs", func() {
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{
				"2001:db8::/32",
				"fd00::/8",
			}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("denies creation when an entry is a plain IP address", func() {
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"192.168.1.1"}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("all LoadBalancer source range entries must be valid CIDR"))
		})

		It("denies creation when an entry is an arbitrary string", func() {
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"not-a-cidr"}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("all LoadBalancer source range entries must be valid CIDR"))
		})

		It("denies creation when at least one entry in a mixed list is invalid", func() {
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{
				"10.0.0.0/8",
				"not-a-cidr",
			}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("all LoadBalancer source range entries must be valid CIDR"))
		})
	})

	Context("dnsServiceIPs", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = "10.96.0.0/16"
		})

		It("allows creation when dnsServiceIPs is not set", func() {
			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with an explicitly empty dnsServiceIPs list", func() {
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation when all IPs are within the service CIDR", func() {
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{"10.96.0.10"}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("allows creation with multiple IPs all within the service CIDR", func() {
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.96.0.10",
				"10.96.0.11",
			}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("denies creation when a DNS service IP is outside the service CIDR", func() {
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{"192.168.1.10"}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("all DNS service IPs must be part of the Service CIDR"))
		})

		It("denies creation when at least one IP in a mixed list is outside the service CIDR", func() {
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.96.0.10",
				"192.168.1.10",
			}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("all DNS service IPs must be part of the Service CIDR"))
		})

		It("allows creation with an IPv6 DNS service IP within an IPv6 service CIDR", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = ipv6CIDRBlock
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{"fd00::10"}

			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
		})

		It("denies creation when an IPv6 DNS service IP is outside the IPv6 service CIDR", func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = ipv6CIDRBlock
			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{"2001:db8::10"}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("all DNS service IPs must be part of the Service CIDR"))
		})
	})
})
