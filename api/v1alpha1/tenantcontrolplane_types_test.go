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
})
