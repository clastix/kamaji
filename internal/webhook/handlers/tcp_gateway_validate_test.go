// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

// Mock discovery client for testing.
type mockDiscoveryClient struct {
	discovery.DiscoveryInterface
	serverGroups         *metav1.APIGroupList
	serverGroupsError    error
	serverResources      map[string]*metav1.APIResourceList
	serverResourcesError map[string]error
}

func (m *mockDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	return m.serverGroups, m.serverGroupsError
}

func (m *mockDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	if err, exists := m.serverResourcesError[groupVersion]; exists {
		return nil, err
	}
	if resources, exists := m.serverResources[groupVersion]; exists {
		return resources, nil
	}

	return &metav1.APIResourceList{}, nil
}

var _ = Describe("TCP Gateway Validation Webhook", func() {
	var (
		ctx           context.Context
		handler       handlers.TenantControlPlaneGatewayValidation
		tcp           *kamajiv1alpha1.TenantControlPlane
		mockClient    client.Client
		mockDiscovery *mockDiscoveryClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = nil
		mockDiscovery = &mockDiscoveryClient{
			serverResources:      make(map[string]*metav1.APIResourceList),
			serverResourcesError: make(map[string]error),
		}

		handler = handlers.TenantControlPlaneGatewayValidation{
			Client:          mockClient,
			DiscoveryClient: mockDiscovery,
		}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{},
		}
	})

	Context("when TenantControlPlane has no Gateway configuration", func() {
		It("should allow creation without Gateway APIs", func() {
			mockDiscovery.serverGroups = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{},
			}

			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow creation with Gateway APIs available", func() {
			mockDiscovery.serverGroups = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{Name: "gateway.networking.k8s.io"},
				},
			}

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

		Context("and Gateway APIs are available", func() {
			BeforeEach(func() {
				mockDiscovery.serverGroups = &metav1.APIGroupList{
					Groups: []metav1.APIGroup{
						{Name: "gateway.networking.k8s.io"},
					},
				}
				mockDiscovery.serverResources["gateway.networking.k8s.io/v1alpha2"] = &metav1.APIResourceList{
					APIResources: []metav1.APIResource{
						{Kind: "TLSRoute"},
					},
				}
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

		Context("and Gateway APIs are not available", func() {
			BeforeEach(func() {
				mockDiscovery.serverGroups = &metav1.APIGroupList{
					Groups: []metav1.APIGroup{},
				}
			})

			It("should deny creation with clear error message", func() {
				_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Gateway API is not available in this cluster"))
			})

			It("should deny updates with clear error message", func() {
				oldTCP := tcp.DeepCopy()
				_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Gateway API is not available in this cluster"))
			})
		})

		Context("and Gateway API group exists but TLSRoute is not available", func() {
			BeforeEach(func() {
				mockDiscovery.serverGroups = &metav1.APIGroupList{
					Groups: []metav1.APIGroup{
						{Name: "gateway.networking.k8s.io"},
					},
				}
				mockDiscovery.serverResources["gateway.networking.k8s.io/v1alpha2"] = &metav1.APIResourceList{
					APIResources: []metav1.APIResource{
						{Kind: "Gateway"},
						{Kind: "HTTPRoute"},
					},
				}
			})

			It("should deny creation when TLSRoute is missing", func() {
				_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("TLSRoute resource is not available"))
			})
		})
	})

	Context("when Gateway configuration is added in update", func() {
		It("should validate Gateway APIs when adding Gateway configuration", func() {
			oldTCP := tcp.DeepCopy()

			tcp.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}

			mockDiscovery.serverGroups = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{},
			}

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Gateway API is not available"))
		})

		It("should allow removing Gateway configuration", func() {
			// Start with Gateway configuration
			oldTCP := tcp.DeepCopy()
			oldTCP.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}

			mockDiscovery.serverGroups = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{},
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

			mockDiscovery.serverGroups = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{},
			}

			admissionResponse := handler.OnDelete(tcp)
			_, err := admissionResponse(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("with different Gateway API versions", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Gateway = &kamajiv1alpha1.GatewaySpec{
				Hostname: gatewayv1.Hostname("api.example.com"),
			}
			mockDiscovery.serverGroups = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{Name: "gateway.networking.k8s.io"},
				},
			}
		})

		It("should work with v1alpha2 TLSRoute", func() {
			mockDiscovery.serverResources["gateway.networking.k8s.io/v1alpha2"] = &metav1.APIResourceList{
				APIResources: []metav1.APIResource{
					{Kind: "TLSRoute"},
				},
			}

			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle missing version gracefully", func() {
			_, err := handler.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("TLSRoute resource is not available"))
		})
	})
})
