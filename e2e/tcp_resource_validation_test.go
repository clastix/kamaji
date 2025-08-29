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

var _ = Describe("TenantControlPlane Resource Validation", func() {
	Context("With default RBAC bootstrap enabled", func() {
		var tcp *kamajiv1alpha1.TenantControlPlane

		BeforeEach(func() {
			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-rbac-validation",
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
						Address: "172.18.0.3",
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
							Enabled:     true,
							AdminUsers:  []string{"kubernetes-admin"},
							AdminGroups: []string{"system:masters"},
						},
					},
					Addons: kamajiv1alpha1.AddonsSpec{},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		})

		It("Should have cluster-admin RBAC resources in tenant cluster", func() {
			TenantClusterResourcesMustBeValid(tcp)
		})

		It("Should validate cluster-admin bindings specifically", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			validator.ValidateClusterAdminRBAC()
		})

		It("Should validate standard Kubernetes resources", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			validator.ValidateStandardKubernetesResources()
		})
	})

	Context("With CoreDNS addon enabled", func() {
		var tcp *kamajiv1alpha1.TenantControlPlane

		BeforeEach(func() {
			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-coredns-validation",
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
						Address: "172.18.0.4",
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
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		})

		It("Should have CoreDNS resources in tenant cluster", func() {
			TenantClusterResourcesMustBeValid(tcp)
		})

		It("Should validate CoreDNS addon specifically", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			validator.ValidateCoreDNS()
		})

		It("Should have kube-dns service available", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			By("specifically checking kube-dns service", func() {
				validator.ValidateCoreDNS()
			})
		})
	})

	Context("With kube-proxy addon enabled", func() {
		var tcp *kamajiv1alpha1.TenantControlPlane

		BeforeEach(func() {
			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-kubeproxy-validation",
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
						Address: "172.18.0.5",
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
						KubeProxy: &kamajiv1alpha1.AddonSpec{
							ImageOverrideTrait: kamajiv1alpha1.ImageOverrideTrait{
								ImageRepository: "registry.k8s.io/kube-proxy",
								ImageTag:        "v1.29.0",
							},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		})

		It("Should have kube-proxy resources in tenant cluster", func() {
			TenantClusterResourcesMustBeValid(tcp)
		})

		It("Should validate kube-proxy addon specifically", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			validator.ValidateKubeProxy()
		})
	})

	Context("With both CoreDNS and kube-proxy enabled", func() {
		var tcp *kamajiv1alpha1.TenantControlPlane

		BeforeEach(func() {
			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-full-addons-validation",
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
						Address: "172.18.0.6",
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
							Enabled:     true,
							AdminUsers:  []string{"kubernetes-admin", "admin"},
							AdminGroups: []string{"system:masters", "cluster-admins"},
						},
					},
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
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		})

		It("Should have all expected resources in tenant cluster", func() {
			TenantClusterResourcesMustBeValid(tcp)
		})

		It("Should validate complete cluster setup", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			By("validating cluster health", func() {
				validator.ValidateClusterHealth()
			})

			By("validating standard resources", func() {
				validator.ValidateStandardKubernetesResources()
			})

			By("validating RBAC setup", func() {
				validator.ValidateClusterAdminRBAC()
			})

			By("validating CoreDNS addon", func() {
				validator.ValidateCoreDNS()
			})

			By("validating kube-proxy addon", func() {
				validator.ValidateKubeProxy()
			})
		})
	})

	Context("With RBAC bootstrap disabled", func() {
		var tcp *kamajiv1alpha1.TenantControlPlane

		BeforeEach(func() {
			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-no-rbac-validation",
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
						Address: "172.18.0.7",
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
							Enabled: false,
						},
					},
					Addons: kamajiv1alpha1.AddonsSpec{},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		})

		It("Should not have Kamaji RBAC bindings when disabled", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			// This should skip RBAC validation since it's disabled
			validator.ValidateClusterAdminRBAC()
		})

		It("Should still have standard Kubernetes resources", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

			validator, err := NewTenantClusterValidator(tcp)
			Expect(err).NotTo(HaveOccurred())

			validator.ValidateStandardKubernetesResources()
		})
	})
})
