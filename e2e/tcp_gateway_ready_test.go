// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with Gateway API", func() {
	// Fill TenantControlPlane object with Gateway configuration
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-gateway",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
				},
				GatewayRoutes: &kamajiv1alpha1.GatewayRoutesSpec{
					Hostnames: []gatewayv1.Hostname{"tcp-gateway.example.com"},
					AdditionalMetadata: kamajiv1alpha1.AdditionalMetadata{
						Labels: map[string]string{
							"test.kamaji.io/gateway": "true",
						},
						Annotations: map[string]string{
							"test.kamaji.io/created-by": "e2e-test",
						},
					},
					GatewayParentRefs: []gatewayv1.ParentReference{
						{
							Name: "test-gateway",
						},
					},
				},
			},
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				Address: "172.18.0.3",
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.23.6",
				Kubelet: kamajiv1alpha1.KubeletSpec{
					CGroupFS: "cgroupfs",
				},
				AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
					"LimitRanger",
					"ResourceQuota",
				},
			},
			Addons: kamajiv1alpha1.AddonsSpec{},
		},
	}

	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	// Check if TenantControlPlane resource has been created
	It("Should be Ready", func() {
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
	})

	// Check if HTTPRoute has been created
	It("Should create HTTPRoute resource", func() {
		Eventually(func() error {
			httpRoute := &gatewayv1.HTTPRoute{}
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, httpRoute)
		}, "2m", "10s").Should(Succeed())
	})

	// Check if GRPCRoute has been created
	It("Should create GRPCRoute resource", func() {
		Eventually(func() error {
			grpcRoute := &gatewayv1.GRPCRoute{}
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, grpcRoute)
		}, "2m", "10s").Should(Succeed())
	})

	// // Check if HTTPRoute has correct configuration
	// It("Should have correct HTTPRoute configuration", func() {
	// 	var httpRoute *gatewayv1.HTTPRoute

	// 	Eventually(func() error {
	// 		httpRoute = &gatewayv1.HTTPRoute{}
	// 		return k8sClient.Get(context.Background(), types.NamespacedName{
	// 			Name:      tcp.Name,
	// 			Namespace: tcp.Namespace,
	// 		}, httpRoute)
	// 	}, "5m", "10s").Should(Succeed())

	// 	// Check labels
	// 	Expect(httpRoute.Labels).To(HaveKeyWithValue("test.kamaji.io/gateway", "true"))
	// 	Expect(httpRoute.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "kamaji"))

	// 	// Check annotations
	// 	Expect(httpRoute.Annotations).To(HaveKeyWithValue("test.kamaji.io/created-by", "e2e-test"))

	// 	// Check parent references
	// 	Expect(httpRoute.Spec.ParentRefs).To(HaveLen(1))
	// 	Expect(httpRoute.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("test-gateway")))

	// 	// Check hostname
	// 	Expect(httpRoute.Spec.Hostnames).To(ContainElement(gatewayv1.Hostname("tcp-gateway.example.com")))

	// 	// Check that it has backend rules
	// 	Expect(httpRoute.Spec.Rules).To(HaveLen(1))
	// 	Expect(httpRoute.Spec.Rules[0].BackendRefs).To(HaveLen(1))
	// })

	// // Check if GRPCRoute has correct configuration
	// It("Should have correct GRPCRoute configuration", func() {
	// 	var grpcRoute *gatewayv1.GRPCRoute

	// 	Eventually(func() error {
	// 		grpcRoute = &gatewayv1.GRPCRoute{}
	// 		return k8sClient.Get(context.Background(), types.NamespacedName{
	// 			Name:      tcp.Name,
	// 			Namespace: tcp.Namespace,
	// 		}, grpcRoute)
	// 	}, "5m", "10s").Should(Succeed())

	// 	// Check labels
	// 	Expect(grpcRoute.Labels).To(HaveKeyWithValue("test.kamaji.io/gateway", "true"))
	// 	Expect(grpcRoute.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "kamaji"))

	// 	// Check annotations
	// 	Expect(grpcRoute.Annotations).To(HaveKeyWithValue("test.kamaji.io/created-by", "e2e-test"))

	// 	// Check parent references
	// 	Expect(grpcRoute.Spec.ParentRefs).To(HaveLen(1))
	// 	Expect(grpcRoute.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("test-gateway")))

	// 	// Check hostname
	// 	Expect(grpcRoute.Spec.Hostnames).To(ContainElement(gatewayv1.Hostname("tcp-gateway.example.com")))

	// 	// Check that it has backend rules
	// 	Expect(grpcRoute.Spec.Rules).To(HaveLen(1))
	// 	Expect(grpcRoute.Spec.Rules[0].BackendRefs).To(HaveLen(1))
	// })

	// Check cleanup when Gateway configuration is removed
	Context("When Gateway configuration is removed", func() {
		It("Should cleanup Gateway resources", func() {
			// Wait for resources to be created first
			Eventually(func() error {
				httpRoute := &gatewayv1.HTTPRoute{}
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      tcp.Name,
					Namespace: tcp.Namespace,
				}, httpRoute)
			}, "5m", "10s").Should(Succeed())

			// Update TCP to remove Gateway configuration
			Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), tcp)).To(Succeed())
			tcp.Spec.ControlPlane.GatewayRoutes = nil
			Expect(k8sClient.Update(context.Background(), tcp)).To(Succeed())

			// Check that HTTPRoute is deleted
			Eventually(func() bool {
				httpRoute := &gatewayv1.HTTPRoute{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      tcp.Name,
					Namespace: tcp.Namespace,
				}, httpRoute)
				return err != nil
			}, "2m", "5s").Should(BeTrue())

			// Check that GRPCRoute is deleted
			Eventually(func() bool {
				grpcRoute := &gatewayv1.GRPCRoute{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      tcp.Name,
					Namespace: tcp.Namespace,
				}, grpcRoute)
				return err != nil
			}, "2m", "5s").Should(BeTrue())
		})
	})
})
