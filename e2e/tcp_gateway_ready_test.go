// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
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

var _ = Describe("Deploy a TenantControlPlane with Gateway API", func() {
	var tcp *kamajiv1alpha1.TenantControlPlane

	JustBeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
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
					Gateway: &kamajiv1alpha1.GatewaySpec{
						Hostname: gatewayv1.Hostname("tcp-gateway.example.com"),
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

	It("Should create TLSRoute resource", func() {
		Eventually(func() error {
			route := &gatewayv1alpha2.TLSRoute{}
			// TODO: Check ownership.
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, route)
		}).WithTimeout(time.Minute).Should(Succeed())
	})
})
