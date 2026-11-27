// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

var _ = Describe("TCP reserved container names Validation Webhook", func() {
	var (
		ctx     context.Context
		handler handlers.TenantControlPlaneReservedContainerNames
		tcp     *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()

		handler = handlers.TenantControlPlaneReservedContainerNames{}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
		}
	})

	Context("on creation", func() {
		It("should allow additional containers with non-reserved names", func() {
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{Name: "sidecar"}}
			tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = []corev1.Container{{Name: "init-sidecar"}}

			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow overriding the kine container and the chmod init container", func() {
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{Name: "kine", Image: "ghcr.io/k3s-io/kine:v0.17.6"}}
			tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = []corev1.Container{{Name: "chmod", Image: "ghcr.io/k3s-io/kine:v0.17.6"}}

			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("should deny an additional container with a reserved name",
			func(name string) {
				tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{Name: "sidecar"}, {Name: name}}

				_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
				Expect(err).To(MatchError(ContainSubstring(`spec.controlPlane.deployment.additionalContainers[1].name: "` + name + `" is reserved`)))
			},
			Entry("kube-apiserver", "kube-apiserver"),
			Entry("kube-controller-manager", "kube-controller-manager"),
			Entry("kube-scheduler", "kube-scheduler"),
			Entry("konnectivity-server", "konnectivity-server"),
			Entry("chmod, the kine init container", "chmod"),
		)

		DescribeTable("should deny an additional init container with a reserved name",
			func(name string) {
				tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = []corev1.Container{{Name: name}}

				_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
				Expect(err).To(MatchError(ContainSubstring(`spec.controlPlane.deployment.additionalInitContainers[0].name: "` + name + `" is reserved`)))
			},
			Entry("kube-apiserver", "kube-apiserver"),
			Entry("kube-controller-manager", "kube-controller-manager"),
			Entry("kube-scheduler", "kube-scheduler"),
			Entry("konnectivity-server", "konnectivity-server"),
			Entry("kine", "kine"),
		)
	})

	Context("on update", func() {
		It("should deny introducing a reserved name", func() {
			oldTCP := tcp.DeepCopy()
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{Name: "kube-apiserver", Image: "mocked"}}

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
			Expect(err).To(MatchError(ContainSubstring(`"kube-apiserver" is reserved`)))
		})

		It("should allow updates to a TenantControlPlane already declaring a reserved name", func() {
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{Name: "kube-apiserver", Image: "mocked"}}
			oldTCP := tcp.DeepCopy()
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = append(tcp.Spec.ControlPlane.Deployment.AdditionalContainers, corev1.Container{Name: "sidecar"})

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
