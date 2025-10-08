// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func createTenantClientset(config *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(config)
}

var _ = Describe("TenantControlPlane with PublicAPIServerAddress", func() {
	var (
		ctx context.Context
		tcp *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("cluster-info ConfigMap generation", func() {
		var publicAddress string

		BeforeEach(func() {
			publicAddress = "k8s-api.test.example.com"

			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-public-address-test",
					Namespace: "default",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							Replicas: pointer.To(int32(1)),
						},
						Service: kamajiv1alpha1.ServiceSpec{
							ServiceType:            kamajiv1alpha1.ServiceTypeNodePort,
							PublicAPIServerAddress: publicAddress,
						},
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: GetKindIPAddress(),
						Port:    31444, // Use different port to avoid conflicts
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.29.0",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							CGroupFS: "cgroupfs",
						},
						AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
							"LimitRanger",
							"ResourceQuota",
						},
					},
					DataStore: "default",
				},
			}
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, tcp)).Should(Succeed())
		})

		It("should create cluster-info ConfigMap with public address", func() {
			By("creating the TenantControlPlane")
			Expect(k8sClient.Create(ctx, tcp)).Should(Succeed())

			By("waiting for the TenantControlPlane to be ready")
			Eventually(func() error {
				namespacedName := types.NamespacedName{
					Name:      tcp.GetName(),
					Namespace: tcp.GetNamespace(),
				}

				if err := k8sClient.Get(ctx, namespacedName, tcp); err != nil {
					return err
				}

				if len(tcp.Status.ControlPlaneEndpoint) == 0 {
					return fmt.Errorf("control plane endpoint not yet available")
				}

				return nil
			}, "60s", "1s").Should(Succeed())

			By("checking that PublicControlPlaneAddress returns the public address")
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal(publicAddress))
			Expect(port).To(Equal(int32(31444)))

			By("getting tenant kubeconfig and checking for bootstrap tokens")
			Eventually(func() error {
				// Get the admin kubeconfig secret
				kubeconfigSecret := &corev1.Secret{}
				secretName := types.NamespacedName{
					Name:      fmt.Sprintf("%s-admin-kubeconfig", tcp.GetName()),
					Namespace: "kamaji-system",
				}

				if err := k8sClient.Get(ctx, secretName, kubeconfigSecret); err != nil {
					return fmt.Errorf("failed to get kubeconfig secret: %w", err)
				}

				kubeconfigData, exists := kubeconfigSecret.Data["admin.conf"]
				if !exists {
					return fmt.Errorf("admin.conf not found in kubeconfig secret")
				}

				// Parse the kubeconfig to create a client for the tenant cluster
				config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
				if err != nil {
					return fmt.Errorf("failed to parse kubeconfig: %w", err)
				}

				// Create clientset for the tenant cluster
				tenantClient, err := createTenantClientset(config)
				if err != nil {
					return fmt.Errorf("failed to create tenant client: %w", err)
				}

				// Check cluster-info ConfigMap in tenant cluster
				clusterInfo, err := tenantClient.CoreV1().ConfigMaps(metav1.NamespacePublic).Get(
					ctx,
					bootstrapapi.ConfigMapClusterInfo,
					metav1.GetOptions{},
				)
				if err != nil {
					return fmt.Errorf("failed to get cluster-info ConfigMap: %w", err)
				}

				// Parse the kubeconfig from cluster-info
				kubeconfigRaw, exists := clusterInfo.Data[bootstrapapi.KubeConfigKey]
				if !exists {
					return fmt.Errorf("kubeconfig not found in cluster-info ConfigMap")
				}

				parsedConfig, err := clientcmd.Load([]byte(kubeconfigRaw))
				if err != nil {
					return fmt.Errorf("failed to parse cluster-info kubeconfig: %w", err)
				}

				// Verify the server URL contains our public address
				expectedServerURL := fmt.Sprintf("https://%s:%d", publicAddress, 31444)
				for _, cluster := range parsedConfig.Clusters {
					if cluster.Server != expectedServerURL {
						return fmt.Errorf("expected server URL %s, got %s", expectedServerURL, cluster.Server)
					}
				}

				return nil
			}, "120s", "5s").Should(Succeed())
		})

		It("should fall back to assigned address when PublicAPIServerAddress is empty", func() {
			By("creating TenantControlPlane without PublicAPIServerAddress")
			tcp.Spec.ControlPlane.Service.PublicAPIServerAddress = ""
			Expect(k8sClient.Create(ctx, tcp)).Should(Succeed())

			By("waiting for the TenantControlPlane to be ready")
			Eventually(func() error {
				namespacedName := types.NamespacedName{
					Name:      tcp.GetName(),
					Namespace: tcp.GetNamespace(),
				}

				if err := k8sClient.Get(ctx, namespacedName, tcp); err != nil {
					return err
				}

				if len(tcp.Status.ControlPlaneEndpoint) == 0 {
					return fmt.Errorf("control plane endpoint not yet available")
				}

				return nil
			}, "60s", "1s").Should(Succeed())

			By("checking that PublicControlPlaneAddress falls back to assigned address")
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())

			// Should use the assigned address from NetworkProfile.Address
			assignedAddress, assignedPort, err := tcp.AssignedControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal(assignedAddress))
			Expect(port).To(Equal(assignedPort))
		})
	})
})
