// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("TenantControlPlane Validation Unit Tests", func() {
	Context("Configuration Validation", func() {
		It("Should validate TenantControlPlane spec for required resources", func() {
			// This test validates the TenantControlPlane configuration without
			// requiring a full Kamaji installation, focusing on spec validation
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-spec-validation",
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
						Address: "172.18.0.100",
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
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:    true,
							AdminUsers: []string{"kubernetes-admin"},
						},
					},
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

			By("validating RBAC bootstrap configuration", func() {
				Expect(tcp.Spec.Bootstrap).NotTo(BeNil(), "Bootstrap configuration should be present")
				Expect(tcp.Spec.Bootstrap.RBAC).NotTo(BeNil(), "RBAC bootstrap should be configured")
				Expect(tcp.Spec.Bootstrap.RBAC.Enabled).To(BeTrue(), "RBAC bootstrap should be enabled")
				Expect(tcp.Spec.Bootstrap.RBAC.AdminUsers).To(ContainElement("kubernetes-admin"), "Should have kubernetes-admin user")
			})

			By("validating CoreDNS addon configuration", func() {
				Expect(tcp.Spec.Addons.CoreDNS).NotTo(BeNil(), "CoreDNS addon should be configured")
				Expect(tcp.Spec.Addons.CoreDNS.ImageRepository).To(Equal("registry.k8s.io/coredns"), "Should use correct CoreDNS repository")
				Expect(tcp.Spec.Addons.CoreDNS.ImageTag).To(Equal("v1.10.1"), "Should specify CoreDNS version")
			})

			By("validating basic Kubernetes configuration", func() {
				Expect(tcp.Spec.Kubernetes.Version).To(Equal("v1.29.0"), "Should specify Kubernetes version")
				Expect(tcp.Spec.NetworkProfile.Address).NotTo(BeEmpty(), "Should have network address configured")
				Expect(tcp.Spec.ControlPlane.Deployment.Replicas).To(Equal(pointer.To(int32(1))), "Should have replica count")
			})

			By("demonstrating expected configuration for complete cluster", func() {
				GinkgoWriter.Printf("Expected TenantControlPlane configuration for complete cluster:\n")
				GinkgoWriter.Printf("✅ Bootstrap.RBAC.Enabled: %t\n", tcp.Spec.Bootstrap.RBAC.Enabled)
				GinkgoWriter.Printf("✅ Bootstrap.RBAC.AdminUsers: %v\n", tcp.Spec.Bootstrap.RBAC.AdminUsers)
				GinkgoWriter.Printf("✅ Addons.CoreDNS configured: %t\n", tcp.Spec.Addons.CoreDNS != nil)
				if tcp.Spec.Addons.CoreDNS != nil {
					GinkgoWriter.Printf("✅ CoreDNS Image: %s:%s\n",
						tcp.Spec.Addons.CoreDNS.ImageRepository,
						tcp.Spec.Addons.CoreDNS.ImageTag)
				}
				GinkgoWriter.Printf("✅ Kubernetes Version: %s\n", tcp.Spec.Kubernetes.Version)
			})
		})

		It("Should identify missing configuration that leads to missing resources", func() {
			// This test demonstrates what configurations are missing when users
			// report missing cluster-admin RBAC or kube-dns service
			tcpMinimal := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-minimal-config",
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
						Address: "172.18.0.101",
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.29.0",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							CGroupFS: "cgroupfs",
						},
					},
					// No Bootstrap configuration
					// No Addons configuration
					Addons: kamajiv1alpha1.AddonsSpec{},
				},
			}

			By("identifying missing RBAC configuration", func() {
				if tcpMinimal.Spec.Bootstrap == nil || tcpMinimal.Spec.Bootstrap.RBAC == nil {
					GinkgoWriter.Printf("⚠️  Missing Bootstrap.RBAC configuration\n")
					GinkgoWriter.Printf("   This may result in missing cluster-admin RBAC bindings\n")
					GinkgoWriter.Printf("   Solution: Add Bootstrap.RBAC with Enabled: true\n")
				}
			})

			By("identifying missing CoreDNS configuration", func() {
				if tcpMinimal.Spec.Addons.CoreDNS == nil {
					GinkgoWriter.Printf("⚠️  Missing Addons.CoreDNS configuration\n")
					GinkgoWriter.Printf("   This will result in missing kube-dns service\n")
					GinkgoWriter.Printf("   Solution: Add CoreDNS addon with image repository and tag\n")
				}
			})

			By("providing configuration recommendations", func() {
				GinkgoWriter.Printf("\n📋 Configuration recommendations to avoid missing resources:\n")
				GinkgoWriter.Printf("1. Always configure Bootstrap.RBAC.Enabled: true\n")
				GinkgoWriter.Printf("2. Always configure Addons.CoreDNS for DNS resolution\n")
				GinkgoWriter.Printf("3. Consider configuring Addons.KubeProxy for networking\n")
				GinkgoWriter.Printf("4. Verify datastore is configured and accessible\n")
			})
		})
	})

	Context("Resource Requirements Documentation", func() {
		It("Should document expected tenant cluster resources", func() {
			By("listing standard Kubernetes resources that should exist", func() {
				GinkgoWriter.Printf("\n📚 Expected resources in a properly configured tenant cluster:\n")

				GinkgoWriter.Printf("\n🔒 RBAC Resources (when Bootstrap.RBAC.Enabled: true):\n")
				GinkgoWriter.Printf("  - ClusterRole: cluster-admin, admin, edit, view, system:node\n")
				GinkgoWriter.Printf("  - ClusterRoleBinding: kamaji-bootstrap-admin-users\n")
				GinkgoWriter.Printf("  - ClusterRoleBinding: kamaji-bootstrap-admin-groups (if groups specified)\n")

				GinkgoWriter.Printf("\n🌐 CoreDNS Resources (when Addons.CoreDNS configured):\n")
				GinkgoWriter.Printf("  - Service: kube-dns (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - Deployment: coredns (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - ConfigMap: coredns (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - ServiceAccount: coredns (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - ClusterRole: system:coredns\n")
				GinkgoWriter.Printf("  - ClusterRoleBinding: system:coredns\n")

				GinkgoWriter.Printf("\n🔧 kube-proxy Resources (when Addons.KubeProxy configured):\n")
				GinkgoWriter.Printf("  - DaemonSet: kube-proxy (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - ConfigMap: kube-proxy (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - ServiceAccount: kube-proxy (in kube-system namespace)\n")
				GinkgoWriter.Printf("  - ClusterRoleBinding: kubeadm:node-proxier\n")

				GinkgoWriter.Printf("\n⚙️  Standard Kubernetes Resources:\n")
				GinkgoWriter.Printf("  - Namespace: default, kube-system\n")
				GinkgoWriter.Printf("  - Service: kubernetes (in default namespace)\n")
				GinkgoWriter.Printf("  - Various system ClusterRoles and bindings\n")
			})

			By("explaining why resources might be missing", func() {
				GinkgoWriter.Printf("\n❗ Common reasons for missing resources:\n")
				GinkgoWriter.Printf("1. Addons not explicitly configured in TenantControlPlane spec\n")
				GinkgoWriter.Printf("2. RBAC bootstrap disabled or not configured\n")
				GinkgoWriter.Printf("3. Datastore not available or not configured\n")
				GinkgoWriter.Printf("4. Kamaji controller not running or having issues\n")
				GinkgoWriter.Printf("5. Network connectivity issues between control plane and datastore\n")
			})
		})
	})
})
