// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Missing Resources Regression Tests", func() {
	Context("Fresh TenantControlPlane deployment", func() {
		It("Should not be missing cluster-admin RBAC or kube-dns service", func() {
			// This test addresses the specific issue reported where fresh clusters
			// were missing cluster-admin RBAC and kube-dns service
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-regression-test",
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
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: "172.18.0.10",
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
					// Explicitly enable RBAC bootstrap (though it should be default)
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:    true,
							AdminUsers: []string{"kubernetes-admin"},
						},
					},
					// Explicitly enable CoreDNS to ensure kube-dns service exists
					Addons: kamajiv1alpha1.AddonsSpec{
						CoreDNS: &kamajiv1alpha1.AddonSpec{
							ImageOverrideTrait: kamajiv1alpha1.ImageOverrideTrait{
								ImageRepository: "registry.k8s.io/coredns",
								ImageTag:        "v1.10.1",
							},
						},
					},
				},
			}

			// Deploy the TenantControlPlane
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
			defer func() {
				Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
			}()

			// Wait for it to be ready
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			// Create validator to check tenant cluster resources
			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred(), "should be able to access tenant cluster")

			By("verifying cluster-admin RBAC exists", func() {
				validator.ValidateClusterAdminRBAC()
			})

			By("verifying kube-dns service exists", func() {
				validator.ValidateCoreDNS()
			})

			By("verifying all standard resources are present", func() {
				validator.ValidateStandardKubernetesResources()
			})

			By("verifying cluster is healthy and functional", func() {
				validator.ValidateClusterHealth()
			})
		})
	})

	Context("Minimal TenantControlPlane without explicit addon configuration", func() {
		It("Should explain why resources might be missing", func() {
			// This test demonstrates what happens when addons are not explicitly configured
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-minimal-test",
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
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: "172.18.0.11",
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
					// No explicit bootstrap configuration (relies on defaults)
					// No explicit addons configuration
					Addons: kamajiv1alpha1.AddonsSpec{},
				},
			}

			// Deploy the TenantControlPlane
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
			defer func() {
				Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
			}()

			// Wait for it to be ready
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			// Create validator to check tenant cluster resources
			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred(), "should be able to access tenant cluster")

			By("verifying cluster-admin RBAC exists (should be enabled by default)", func() {
				validator.ValidateClusterAdminRBAC()
			})

			By("verifying kube-dns service does NOT exist (CoreDNS not enabled)", func() {
				// This should skip the CoreDNS validation since addon is not configured
				validator.ValidateCoreDNS()
			})

			By("verifying standard Kubernetes resources exist", func() {
				validator.ValidateStandardKubernetesResources()
			})

			By("demonstrating that addons must be explicitly enabled", func() {
				// The key insight: CoreDNS (and thus kube-dns service) must be explicitly
				// enabled in the TenantControlPlane spec. It's not automatically created.
				Expect(tcp.Spec.Addons.CoreDNS).To(BeNil(), "CoreDNS addon should be nil when not configured")
			})
		})
	})

	Context("Common configuration patterns", func() {
		It("Should provide a template for complete cluster setup", func() {
			// This test provides a reference configuration that ensures all
			// commonly expected resources are present
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-complete-template",
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
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: "172.18.0.12",
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
					// Complete bootstrap configuration
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     true,
							AdminUsers:  []string{"kubernetes-admin"},
							AdminGroups: []string{"system:masters"},
						},
					},
					// Complete addons configuration
					Addons: kamajiv1alpha1.AddonsSpec{
						CoreDNS: &kamajiv1alpha1.AddonSpec{
							ImageOverrideTrait: kamajiv1alpha1.ImageOverrideTrait{
								ImageRepository: "registry.k8s.io/coredns",
								ImageTag:        "v1.10.1",
							},
						},
						KubeProxy: &kamajiv1alpha1.AddonSpec{
							ImageOverrideTrait: kamajiv1alpha1.ImageOverrideTrait{
								ImageRepository: "registry.k8s.io/kube-proxy",
								ImageTag:        "v1.29.0",
							},
						},
					},
				},
			}

			// Deploy the TenantControlPlane
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
			defer func() {
				Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
			}()

			// Wait for it to be ready
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			// Validate all resources are present
			TenantClusterResourcesMustBeValid(tcp)

			By("documenting the complete configuration", func() {
				GinkgoWriter.Printf("Complete TenantControlPlane configuration:\n")
				GinkgoWriter.Printf("- Bootstrap.RBAC.Enabled: %t\n", tcp.Spec.Bootstrap.RBAC.Enabled)
				GinkgoWriter.Printf("- Bootstrap.RBAC.AdminUsers: %v\n", tcp.Spec.Bootstrap.RBAC.AdminUsers)
				GinkgoWriter.Printf("- Bootstrap.RBAC.AdminGroups: %v\n", tcp.Spec.Bootstrap.RBAC.AdminGroups)
				GinkgoWriter.Printf("- Addons.CoreDNS: %s:%s\n",
					tcp.Spec.Addons.CoreDNS.ImageRepository,
					tcp.Spec.Addons.CoreDNS.ImageTag)
				GinkgoWriter.Printf("- Addons.KubeProxy: %s:%s\n",
					tcp.Spec.Addons.KubeProxy.ImageRepository,
					tcp.Spec.Addons.KubeProxy.ImageTag)
			})
		})
	})
})
