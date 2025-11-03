// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
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

var (
	runtimeScheme *runtime.Scheme
)

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

	Describe("Status Update", func() {
		var (
			//gwStatus    *gatewayv1.GatewayStatus
			route gatewayv1alpha2.TLSRouteStatus
			tcpGw *kamajiv1alpha1.KubernetesGatewayStatus
		)
		BeforeEach(func() {
			route = gatewayv1alpha2.TLSRouteStatus{}
			tcpGw = &kamajiv1alpha1.KubernetesGatewayStatus{}
		})
		When("Route status is empty", func() {
			route = gatewayv1alpha2.TLSRouteStatus{}
			It("returns condition unknown", func() {
				err := resources.CheckStatus(route, tcpGw)
				Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(tcpGw.Conditions, "Ready")
				Expect(cond).NotTo(BeNil())
				Expect(cond.Status).To(Equal(metav1.ConditionUnknown))
				Expect(cond.Reason).To(Equal("MissingParentStatus"))
				Expect(cond.Message).To(Equal("The route has no parents status"))
			})
		})
		When("More than one route status", func() {
			route = gatewayv1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1alpha2.RouteStatus{
					Parents: []gatewayv1alpha2.RouteParentStatus{
						{}, {},
					},
				},
			}
			It("returns condition unknown", func() {
				err := resources.CheckStatus(route, tcpGw)
				Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(tcpGw.Conditions, "Ready")
				if Expect(cond).NotTo(BeNil()) {
					Expect(cond.Status).To(Equal(metav1.ConditionUnknown))
					Expect(cond.Reason).To(Equal("MissingParentStatus"))
					Expect(cond.Message).To(Equal("The route has no parents status"))
				}
			})
		})
		When("The route is accepted")
	})

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
})
