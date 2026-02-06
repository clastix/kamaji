// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with Gateway API and Konnectivity", func() {
	var tcp *kamajiv1alpha1.TenantControlPlane

	JustBeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp-konnectivity-gateway",
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
					Gateway: &kamajiv1alpha1.GatewaySpec{
						Hostname: gatewayv1.Hostname("tcp-gateway-konnectivity.example.com"),
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
								Name:        "test-gateway",
								Port:        pointer.To(gatewayv1.PortNumber(6443)),
								SectionName: pointer.To(gatewayv1.SectionName("cp-listener")),
							},
						},
					},
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					Address: "172.18.0.4",
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: "v1.28.0",
					Kubelet: kamajiv1alpha1.KubeletSpec{
						CGroupFS: "cgroupfs",
					},
					AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
						"LimitRanger",
						"ResourceQuota",
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
		}
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())

		// Wait for the object to be completely deleted
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, &kamajiv1alpha1.TenantControlPlane{})

			return err != nil // Returns true when object is not found (deleted)
		}).WithTimeout(time.Minute).Should(BeTrue())
	})

	It("Should be Ready", func() {
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
	})

	It("Should create control plane TLSRoute preserving user-provided parentRef fields", func() {
		Eventually(func() error {
			route := &gatewayv1alpha2.TLSRoute{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, route); err != nil {
				return err
			}
			if len(route.Spec.ParentRefs) == 0 {
				return fmt.Errorf("parentRefs is empty")
			}
			if route.Spec.ParentRefs[0].SectionName == nil {
				return fmt.Errorf("sectionName is nil")
			}
			if *route.Spec.ParentRefs[0].SectionName != gatewayv1.SectionName("cp-listener") {
				return fmt.Errorf("expected sectionName 'cp-listener', got '%s'", *route.Spec.ParentRefs[0].SectionName)
			}
			if route.Spec.ParentRefs[0].Port == nil {
				return fmt.Errorf("port is nil")
			}
			if *route.Spec.ParentRefs[0].Port != gatewayv1.PortNumber(6443) {
				return fmt.Errorf("expected port 6443, got '%d'", *route.Spec.ParentRefs[0].Port)
			}

			return nil
		}).WithTimeout(time.Minute).Should(Succeed())
	})

	It("Should create Konnectivity TLSRoute with correct sectionName", func() {
		Eventually(func() error {
			route := &gatewayv1alpha2.TLSRoute{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name + "-konnectivity",
				Namespace: tcp.Namespace,
			}, route); err != nil {
				return err
			}
			if len(route.Spec.ParentRefs) == 0 {
				return fmt.Errorf("parentRefs is empty")
			}
			if route.Spec.ParentRefs[0].SectionName == nil {
				return fmt.Errorf("sectionName is nil")
			}
			if *route.Spec.ParentRefs[0].SectionName != gatewayv1.SectionName("konnectivity-server") {
				return fmt.Errorf("expected sectionName 'konnectivity-server', got '%s'", *route.Spec.ParentRefs[0].SectionName)
			}

			return nil
		}).WithTimeout(time.Minute).Should(Succeed())
	})

	It("Should use same hostname for both TLSRoutes", func() {
		Eventually(func() error {
			controlPlaneRoute := &gatewayv1alpha2.TLSRoute{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, controlPlaneRoute); err != nil {
				return err
			}

			konnectivityRoute := &gatewayv1alpha2.TLSRoute{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name + "-konnectivity",
				Namespace: tcp.Namespace,
			}, konnectivityRoute); err != nil {
				return err
			}

			if len(controlPlaneRoute.Spec.Hostnames) == 0 || len(konnectivityRoute.Spec.Hostnames) == 0 {
				return fmt.Errorf("hostnames are empty")
			}
			if controlPlaneRoute.Spec.Hostnames[0] != konnectivityRoute.Spec.Hostnames[0] {
				return fmt.Errorf("hostnames do not match: control plane '%s', konnectivity '%s'",
					controlPlaneRoute.Spec.Hostnames[0], konnectivityRoute.Spec.Hostnames[0])
			}

			return nil
		}).WithTimeout(time.Minute).Should(Succeed())
	})
})
