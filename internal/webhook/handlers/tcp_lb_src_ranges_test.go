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

var _ = Describe("TCP LoadBalancer Source Ranges Webhook", func() {
	var (
		ctx context.Context
		t   handlers.TenantControlPlaneLoadBalancerSourceRanges
		tcp *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		t = handlers.TenantControlPlaneLoadBalancerSourceRanges{}
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{},
		}
		ctx = context.Background() //nolint:fatcontext
	})

	It("allows creation when valid CIDR ranges are provided", func() {
		tcp.Spec.ControlPlane.Service.ServiceType = kamajiv1alpha1.ServiceTypeLoadBalancer
		tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"192.168.0.0/24"}
		_, err := t.OnCreate(tcp)(ctx, admission.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("allows creation when LoadBalancer service has no CIDR field", func() {
		tcp.Spec.ControlPlane.Service.ServiceType = kamajiv1alpha1.ServiceTypeLoadBalancer
		_, err := t.OnCreate(tcp)(ctx, admission.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("allows creation when LoadBalancer service has an empty CIDR list", func() {
		tcp.Spec.ControlPlane.Service.ServiceType = kamajiv1alpha1.ServiceTypeLoadBalancer
		tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{}
		_, err := t.OnCreate(tcp)(ctx, admission.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("denies creation when source ranges contain invalid CIDRs", func() {
		tcp.Spec.ControlPlane.Service.ServiceType = kamajiv1alpha1.ServiceTypeLoadBalancer
		tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = []string{"192.168.0.0/33"}
		_, err := t.OnCreate(tcp)(ctx, admission.Request{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid LoadBalancer source CIDR 192.168.0.0/33"))
	})
})
