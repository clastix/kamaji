// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

func TestKonnectivityGatewayResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Konnectivity Gateway Resource Suite")
}

var runtimeScheme *runtime.Scheme

var _ = BeforeSuite(func() {
	runtimeScheme = runtime.NewScheme()
	Expect(scheme.AddToScheme(runtimeScheme)).To(Succeed())
	Expect(kamajiv1alpha1.AddToScheme(runtimeScheme)).To(Succeed())
	Expect(gatewayv1alpha2.Install(runtimeScheme)).To(Succeed())
})

var _ = Describe("KubernetesKonnectivityGatewayResource", func() {
	var (
		tcp      *kamajiv1alpha1.TenantControlPlane
		resource *konnectivity.KubernetesKonnectivityGatewayResource
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient := fake.NewClientBuilder().
			WithScheme(runtimeScheme).
			Build()

		resource = &konnectivity.KubernetesKonnectivityGatewayResource{
			Client: fakeClient,
		}

		namespace := gatewayv1.Namespace("default")
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Gateway: &kamajiv1alpha1.GatewaySpec{
						Hostname: gatewayv1alpha2.Hostname("test.example.com"),
						GatewayParentRefs: []gatewayv1alpha2.ParentReference{
							{
								Name:      "test-gateway",
								Namespace: &namespace,
							},
						},
					},
				},
				Addons: kamajiv1alpha1.AddonsSpec{
					Konnectivity: &kamajiv1alpha1.KonnectivitySpec{
						KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
							Port: 8132,
						},
					},
				},
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Addons: kamajiv1alpha1.AddonsStatus{
					Konnectivity: kamajiv1alpha1.KonnectivityStatus{
						Service: kamajiv1alpha1.KubernetesServiceStatus{
							Name: "test-konnectivity-service",
							Port: 8132,
						},
					},
				},
			},
		}
	})

	Describe("shouldHaveGateway logic", func() {
		It("should return false when Konnectivity addon is disabled", func() {
			tcp.Spec.Addons.Konnectivity = nil
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeFalse())
			Expect(resource.ShouldCleanup(tcp)).To(BeFalse())
		})

		It("should return false when control plane gateway is not configured", func() {
			tcp.Spec.ControlPlane.Gateway = nil
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeFalse())
			Expect(resource.ShouldCleanup(tcp)).To(BeFalse())
		})

		It("should return true when both Konnectivity and gateway are configured", func() {
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeTrue())
			Expect(resource.ShouldCleanup(tcp)).To(BeFalse())
		})
	})

	Context("When Konnectivity gateway should be configured", func() {
		It("should set correct TLSRoute name with -konnectivity suffix", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			route := &gatewayv1alpha2.TLSRoute{}
			err = resource.Client.Get(ctx, client.ObjectKey{Name: "test-tcp-konnectivity", Namespace: tcp.Namespace}, route)
			Expect(err).NotTo(HaveOccurred())
			Expect(route.Name).To(Equal("test-tcp-konnectivity"))
		})

		It("should set sectionName to \"konnectivity-server\" and port from Konnectivity service status", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			route := &gatewayv1alpha2.TLSRoute{}
			err = resource.Client.Get(ctx, client.ObjectKey{Name: "test-tcp-konnectivity", Namespace: tcp.Namespace}, route)
			Expect(err).NotTo(HaveOccurred())
			Expect(route.Spec.ParentRefs).To(HaveLen(1))
			Expect(route.Spec.ParentRefs[0].SectionName).NotTo(BeNil())
			Expect(*route.Spec.ParentRefs[0].SectionName).To(Equal(gatewayv1.SectionName("konnectivity-server")))
			Expect(route.Spec.ParentRefs[0].Port).NotTo(BeNil())
			Expect(*route.Spec.ParentRefs[0].Port).To(Equal(tcp.Status.Addons.Konnectivity.Service.Port))
		})

		It("should use control plane gateway hostname", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			route := &gatewayv1alpha2.TLSRoute{}
			err = resource.Client.Get(ctx, client.ObjectKey{Name: "test-tcp-konnectivity", Namespace: tcp.Namespace}, route)
			Expect(err).NotTo(HaveOccurred())
			Expect(route.Spec.Hostnames).To(HaveLen(1))
			Expect(route.Spec.Hostnames[0]).To(Equal(tcp.Spec.ControlPlane.Gateway.Hostname))
		})
	})

	Context("Konnectivity-specific error cases", func() {
		It("should return early without error when control plane gateway is not configured", func() {
			tcp.Spec.ControlPlane.Gateway = nil

			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			result, err := resource.CreateOrUpdate(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(controllerutil.OperationResultNone))
		})

		It("should fail when Konnectivity service is not ready", func() {
			tcp.Status.Addons.Konnectivity.Service.Name = ""
			tcp.Status.Addons.Konnectivity.Service.Port = 0

			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("konnectivity service not ready"))
		})

		It("should fail when control plane gateway parentRefs are not specified", func() {
			tcp.Spec.ControlPlane.Gateway.GatewayParentRefs = nil

			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("control plane gateway parentRefs are not specified"))
		})
	})

	Context("When Konnectivity gateway should not be configured", func() {
		BeforeEach(func() {
			tcp.Spec.Addons.Konnectivity = nil
			tcp.Status.Addons.Konnectivity = kamajiv1alpha1.KonnectivityStatus{
				Gateway: &kamajiv1alpha1.KubernetesGatewayStatus{
					AccessPoints: nil,
				},
			}
		})

		It("should cleanup when gateway is removed", func() {
			Expect(resource.ShouldCleanup(tcp)).To(BeTrue())
		})
	})

	It("should return correct resource name", func() {
		Expect(resource.GetName()).To(Equal("konnectivity_gateway_routes"))
	})
})
