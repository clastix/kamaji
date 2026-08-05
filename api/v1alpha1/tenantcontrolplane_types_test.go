// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Cluster controller", func() {
	var (
		ctx context.Context
		tcp *TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()
		tcp = &TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp",
				Namespace: "default",
			},
			Spec: TenantControlPlaneSpec{},
		}
	})

	AfterEach(func() {
		if err := k8sClient.Delete(ctx, tcp); err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("LoadBalancer Source Ranges", func() {
		It("allows creation when no CIDR ranges are provided", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeLoadBalancer

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows creation with an explicitly empty CIDR list", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeLoadBalancer
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows creation when service type is not LoadBalancer and it has an empty CIDR list", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows CIDR ranges when service type is LoadBalancer", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeLoadBalancer
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"192.168.0.0/24"}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies CIDR ranges when service type is not LoadBalancer", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort
			tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"192.168.0.0/24"}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("LoadBalancer source ranges are supported only with LoadBalancer service type"))
		})
	})

	Context("AdvertiseAddress", func() {
		It("allows a valid IPv4 address", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort
			tcp.Spec.NetworkProfile.AdvertiseAddress = "10.0.0.10"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows a valid IPv6 address", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort
			tcp.Spec.NetworkProfile.AdvertiseAddress = "2001:db8::1"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows creation when advertiseAddress is unset", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies a DNS hostname", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort
			tcp.Spec.NetworkProfile.AdvertiseAddress = "control-plane.example.com"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("advertiseAddress must be a valid IP address"))
		})

		It("denies a malformed IP address", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort
			tcp.Spec.NetworkProfile.AdvertiseAddress = "10.0.0.999"

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("advertiseAddress must be a valid IP address"))
		})
	})

	Context("AllocateLoadBalancerNodePorts", func() {
		It("allows the field when service type is LoadBalancer", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeLoadBalancer
			tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = new(false)

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows creation when the field is unset and service type is not LoadBalancer", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies the field when service type is not LoadBalancer", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeNodePort
			tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = new(false)

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("allocateLoadBalancerNodePorts is supported only with LoadBalancer service type"))
		})
	})

	Context("IPFamilies", func() {
		It("allows a valid IPv6-only single-stack service", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
			tcp.Spec.ControlPlane.Service.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicySingleStack)
			tcp.Spec.ControlPlane.Service.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows a dual-stack service with two families", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
			tcp.Spec.ControlPlane.Service.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicyRequireDualStack)
			tcp.Spec.ControlPlane.Service.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows creation when the fields are unset", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP

			err := k8sClient.Create(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies SingleStack with two families", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
			tcp.Spec.ControlPlane.Service.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicySingleStack)
			tcp.Spec.ControlPlane.Service.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ipFamilies must contain at most one entry when ipFamilyPolicy is SingleStack"))
		})

		It("denies duplicate families", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
			tcp.Spec.ControlPlane.Service.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv4Protocol}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ipFamilies entries must be unique"))
		})

		It("denies an invalid IP family value", func() {
			tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
			tcp.Spec.ControlPlane.Service.IPFamilies = []corev1.IPFamily{corev1.IPFamily("Foobar")}

			err := k8sClient.Create(ctx, tcp)
			Expect(err).To(HaveOccurred())
			// Actual apiserver error: `spec.controlPlane.service.ipFamilies[0]: Unsupported value: "Foobar": supported values: "IPv4", "IPv6"`
			// "Unsupported value" is the stable enum-rejection phrase from the apiserver.
			Expect(err.Error()).To(ContainSubstring("Unsupported value"))
		})
	})
})
