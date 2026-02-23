// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("TenantControlPlane PublicAPIServerAddress", func() {
	ctx := context.Background()
	var tcp *kamajiv1alpha1.TenantControlPlane

	BeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp-public-address-test",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer,
					},
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: "v1.30.0",
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					Port: 6443,
				},
				DataStore: "default",
			},
		}
	})

	Context("when PublicAPIServerAddress is not specified", func() {
		It("should not set public address in spec", func() {
			Expect(tcp.Spec.ControlPlane.Service.PublicAPIServerAddress).To(BeEmpty())
		})
	})

	Context("when PublicAPIServerAddress is specified", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Service.PublicAPIServerAddress = "k8s-api.example.com"
		})

		It("should set the public address", func() {
			Expect(tcp.Spec.ControlPlane.Service.PublicAPIServerAddress).To(Equal("k8s-api.example.com"))
		})

		It("should create the TenantControlPlane successfully", func() {
			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())

			// Check that the TCP is created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)
			}, 10, 1).Should(Succeed())

			// Check that the public address is returned by PublicControlPlaneAddress
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("k8s-api.example.com"))
			Expect(port).To(Equal(int32(6443)))
		})

		It("should generate kubeconfigs with the public address for controller-manager and scheduler", func() {
			tcp.Spec.ControlPlane.Service.PublicAPIServerAddress = "k8s-api.example.com"
			Expect(k8sClient.Create(ctx, tcp)).To(Succeed())

			// Wait for kubeconfig secrets to be created
			Eventually(func() bool {
				cmSecret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      tcp.Name + "-controller-manager-kubeconfig",
					Namespace: tcp.Namespace,
				}, cmSecret)
				if err != nil {
					return false
				}

				schedSecret := &corev1.Secret{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      tcp.Name + "-scheduler-kubeconfig",
					Namespace: tcp.Namespace,
				}, schedSecret)

				return err == nil
			}, 300, 5).Should(BeTrue())

			// Validate that the kubeconfigs contain the public address in the server URL
			cmSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      tcp.Name + "-controller-manager-kubeconfig",
				Namespace: tcp.Namespace,
			}, cmSecret)).To(Succeed())
			cmConfig, err := clientcmd.Load(cmSecret.Data["controller-manager.conf"])
			Expect(err).NotTo(HaveOccurred())
			Expect(cmConfig.Clusters[cmConfig.CurrentContext].Server).To(Equal("https://k8s-api.example.com:6443"))

			schedSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      tcp.Name + "-scheduler-kubeconfig",
				Namespace: tcp.Namespace,
			}, schedSecret)).To(Succeed())
			schedConfig, err := clientcmd.Load(schedSecret.Data["scheduler.conf"])
			Expect(err).NotTo(HaveOccurred())
			Expect(schedConfig.Clusters[schedConfig.CurrentContext].Server).To(Equal("https://k8s-api.example.com:6443"))
		})
	})
})
