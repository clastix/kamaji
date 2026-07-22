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

var _ = Describe("ServiceCIDRs and PodCIDRs IP families", func() {
	var (
		ctx context.Context
		tcp *TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()
		tcp = &TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "cidr-families-", Namespace: "default"},
			Spec:       TenantControlPlaneSpec{},
		}
		tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
	})

	AfterEach(func() {
		if tcp.GetName() != "" {
			if err := k8sClient.Delete(ctx, tcp); err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	It("accepts a single-stack serviceCidrs", func() {
		tcp.Spec.NetworkProfile.ServiceCIDRs = []string{"10.96.0.0/16"}
		Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
	})

	It("accepts a dual-stack serviceCidrs", func() {
		tcp.Spec.NetworkProfile.ServiceCIDRs = []string{"10.96.0.0/16", "fd00::/108"}
		Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
	})

	It("denies two same-family serviceCidrs", func() {
		tcp.Spec.NetworkProfile.ServiceCIDRs = []string{"10.96.0.0/16", "10.97.0.0/16"}

		err := k8sClient.Create(ctx, tcp)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("serviceCidrs must not contain two CIDRs of the same IP family"))
	})

	It("accepts a dual-stack podCidrs", func() {
		tcp.Spec.NetworkProfile.PodCIDRs = []string{"10.244.0.0/16", "fd00:244::/56"}
		Expect(k8sClient.Create(ctx, tcp)).To(Succeed())
	})

	It("denies two same-family podCidrs", func() {
		tcp.Spec.NetworkProfile.PodCIDRs = []string{"fd00:244::/56", "fd00:245::/56"}

		err := k8sClient.Create(ctx, tcp)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("podCidrs must not contain two CIDRs of the same IP family"))
	})
})
