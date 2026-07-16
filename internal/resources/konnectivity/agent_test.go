// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Agent securityContext", func() {
	var (
		ctx context.Context
		tcp *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					Port: 6443,
				},
				Addons: kamajiv1alpha1.AddonsSpec{
					Konnectivity: &kamajiv1alpha1.KonnectivitySpec{
						KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
							Port: 8132,
						},
						KonnectivityAgentSpec: kamajiv1alpha1.KonnectivityAgentSpec{
							Mode: kamajiv1alpha1.KonnectivityAgentModeDaemonSet,
						},
					},
				},
			},
			// AdvertisedControlPlaneAddress reads Status.ControlPlaneEndpoint
			// (falls through to AssignedControlPlaneAddress).
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				ControlPlaneEndpoint: "192.0.2.1:6443",
				Addons: kamajiv1alpha1.AddonsStatus{
					Konnectivity: kamajiv1alpha1.KonnectivityStatus{
						ClusterRoleBinding: kamajiv1alpha1.ExternalKubernetesObjectStatus{
							Name: "konnectivity-agent-crb",
						},
					},
				},
			},
		}
	})

	When("KonnectivityAgentSpec.SecurityContext is set", func() {
		It("threads the securityContext onto the DaemonSet container", func() {
			sc := &corev1.SecurityContext{
				RunAsNonRoot: func(b bool) *bool { return &b }(true),
				RunAsUser:    func(i int64) *int64 { return &i }(65534),
			}
			tcp.Spec.Addons.Konnectivity.KonnectivityAgentSpec.SecurityContext = sc

			r := &Agent{resource: &appsv1.DaemonSet{}}
			Expect(r.mutate(ctx, tcp)()).To(Succeed())

			ds := r.resource.(*appsv1.DaemonSet) //nolint:forcetypeassert
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].SecurityContext).To(Equal(sc))
		})
	})

	When("KonnectivityAgentSpec.SecurityContext is nil (unset)", func() {
		It("leaves the DaemonSet container SecurityContext nil", func() {
			// SecurityContext is not set on the AgentSpec (nil by default)
			r := &Agent{resource: &appsv1.DaemonSet{}}
			Expect(r.mutate(ctx, tcp)()).To(Succeed())

			ds := r.resource.(*appsv1.DaemonSet) //nolint:forcetypeassert
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].SecurityContext).To(BeNil())
		})
	})
})
