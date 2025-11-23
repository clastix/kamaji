// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

func TestGatewayResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateway Resource Suite")
}

var runtimeScheme *runtime.Scheme

var _ = BeforeSuite(func() {
	runtimeScheme = runtime.NewScheme()
	Expect(scheme.AddToScheme(runtimeScheme)).To(Succeed())
	Expect(kamajiv1alpha1.AddToScheme(runtimeScheme)).To(Succeed())
	Expect(gatewayv1alpha2.Install(runtimeScheme)).To(Succeed())
})

var _ = Describe("KubernetesGatewayResource", func() {
	var (
		tcp      *kamajiv1alpha1.TenantControlPlane
		resource *resources.KubernetesGatewayResource
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient := fake.NewClientBuilder().
			WithScheme(runtimeScheme).
			Build()

		resource = &resources.KubernetesGatewayResource{
			Client: fakeClient,
		}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Gateway: &kamajiv1alpha1.GatewaySpec{
						Hostname: gatewayv1alpha2.Hostname("test.example.com"),
						AdditionalMetadata: kamajiv1alpha1.AdditionalMetadata{
							Labels: map[string]string{
								"test-label": "test-value",
							},
						},
						GatewayParentRefs: []gatewayv1alpha2.ParentReference{
							{
								Name: "test-gateway",
							},
						},
					},
				},
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Kubernetes: kamajiv1alpha1.KubernetesStatus{
					Service: kamajiv1alpha1.KubernetesServiceStatus{
						Name: "test-service",
						Port: 6443,
					},
				},
			},
		}
	})

	Context("When GatewayRoutes is configured", func() {
		It("should not cleanup", func() {
			Expect(resource.ShouldCleanup(tcp)).To(BeFalse())
		})

		It("should define route resources", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should require status update when GatewayRoutes is configured but status is nil", func() {
			tcp.Status.Kubernetes.Gateway = nil
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeTrue())
		})
	})

	Context("When GatewayRoutes is not configured", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Gateway = nil
			tcp.Status.Kubernetes.Gateway = &kamajiv1alpha1.KubernetesGatewayStatus{
				AccessPoints: nil,
			}
		})

		It("should cleanup", func() {
			Expect(resource.ShouldCleanup(tcp)).To(BeTrue())
		})

		It("should not require status update when both spec and status are nil", func() {
			tcp.Status.Kubernetes.Gateway = nil
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeFalse())
		})
	})

	Context("When hostname is missing", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Gateway.Hostname = ""
		})

		It("should fail to create or update", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing hostname"))
		})
	})

	Context("When service is not ready", func() {
		BeforeEach(func() {
			tcp.Status.Kubernetes.Service.Name = ""
			tcp.Status.Kubernetes.Service.Port = 0
		})

		It("should fail to create or update", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("service not ready"))
		})
	})

	It("should return correct resource name", func() {
		Expect(resource.GetName()).To(Equal("gateway_routes"))
	})

	Describe("findMatchingListener", func() {
		var (
			listeners []gatewayv1.Listener
			ref       gatewayv1.ParentReference
		)

		BeforeEach(func() {
			listeners = []gatewayv1.Listener{
				{
					Name: "first",
					Port: gatewayv1.PortNumber(443),
				},
				{
					Name: "middle",
					Port: gatewayv1.PortNumber(6443),
				},
				{
					Name: "last",
					Port: gatewayv1.PortNumber(80),
				},
			}
			ref = gatewayv1.ParentReference{
				Name: "test-gateway",
			}
		})

		It("should return an error when sectionName is nil", func() {
			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing sectionName"))
			Expect(listener).To(Equal(gatewayv1.Listener{}))
		})
		It("should return an error when sectionName is an empty string", func() {
			sectionName := gatewayv1.SectionName("")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find listener ''"))
			Expect(listener).To(Equal(gatewayv1.Listener{}))
		})

		It("should return the matching listener when sectionName points to an existing listener", func() {
			sectionName := gatewayv1.SectionName("middle")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(listener.Port).To(Equal(gatewayv1.PortNumber(6443)))
		})

		It("should return an error when sectionName points to a non-existent listener", func() {
			sectionName := gatewayv1.SectionName("non-existent")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find listener 'non-existent'"))
			Expect(listener).To(Equal(gatewayv1.Listener{}))
		})

		It("should return the first listener", func() {
			sectionName := gatewayv1.SectionName("first")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(listener.Port).To(Equal(gatewayv1.PortNumber(443)))
		})

		It("should return the last listener when matching by name", func() {
			sectionName := gatewayv1.SectionName("last")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(listener.Port).To(Equal(gatewayv1.PortNumber(80)))
		})
	})
})
