// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

var _ = Describe("TCP Gateway Validation Webhook", func() {
	var (
		ctx     context.Context
		handler handlers.TenantControlPlaneGatewayValidation
		tcp     *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()
		handler = handlers.TenantControlPlaneGatewayValidation{}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{},
		}
	})

	Context("when TenantControlPlane has no Gateway configuration", func() {
		It("should allow creation with Gateway API support disabled", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow creation with Gateway API support enabled", func() {
			handler.GatewayAPIEnabled = true

			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when TenantControlPlane has Gateway configuration", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}
		})

		Context("and Gateway API support is enabled", func() {
			BeforeEach(func() {
				handler.GatewayAPIEnabled = true
			})

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

		Context("and Gateway API support is disabled", func() {
			It("should deny creation pointing at the flag", func() {
				_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--enable-gateway-api"))
			})

			It("should deny updates pointing at the flag", func() {
				oldTCP := tcp.DeepCopy()
				_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--enable-gateway-api"))
			})
		})
	})

	Context("when Gateway configuration is added in update", func() {
		It("should deny adding Gateway configuration with Gateway API support disabled", func() {
			oldTCP := tcp.DeepCopy()

			tcp.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("--enable-gateway-api"))
		})

		It("should allow removing Gateway configuration", func() {
			oldTCP := tcp.DeepCopy()
			oldTCP.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("OnDelete operations", func() {
		It("should always allow delete operations", func() {
			tcp.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}

			_, err := handler.OnDelete(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
