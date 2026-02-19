// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy TenantControlPlane addons with custom env vars", func() {
	ctx := context.Background()

	var kubeconfigFile *os.File
	var tcp *kamajiv1alpha1.TenantControlPlane

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(os.Remove(kubeconfigFile.Name())).ToNot(HaveOccurred())
		Expect(k8sClient.Delete(ctx, tcp)).Should(Succeed())
	})

	// Check TenantControlPlane CoreDNS addon
	It("Should handle CoreDNS extra env var configuration", func() {
		By("creating TCP with default CoreDNS addon configuration", func() {
			tcp = CreateKindTCPWithAddons("default", "tcp-coredns-extra-env-var", kamajiv1alpha1.AddonsSpec{CoreDNS: &kamajiv1alpha1.AddonSpec{}})

			Expect(k8sClient.Create(ctx, tcp)).NotTo(HaveOccurred())
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		})

		var clientset *kubernetes.Clientset

		By("getting TCP clientset", func() {
			clientset, kubeconfigFile = GetTenantClientSet(tcp)
		})

		By("checking env vars for default CoreDNS deployment are empty", func() {
			CheckTemplateContainerEnvVars(clientset, "Deployment", "kube-system", "coredns", "coredns", []corev1.EnvVar{}, true)
		})

		extraVars := []corev1.EnvVar{{Name: "MY_VAR", Value: "MY_VALUE"}}

		By("adding extra env vars for CoreDNS", func() {
			updatedTCP := &kamajiv1alpha1.TenantControlPlane{}

			Eventually(func() error {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, updatedTCP)).Should(Succeed())
				updatedTCP.Spec.Addons.CoreDNS = &kamajiv1alpha1.AddonSpec{ExtraEnvs: extraVars}

				return k8sClient.Update(ctx, updatedTCP)
			}).WithTimeout(1 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())

			StatusMustEqualTo(updatedTCP, kamajiv1alpha1.VersionReady)
		})

		By("checking extra env vars for updated CoreDNS deployment are present", func() {
			CheckTemplateContainerEnvVars(clientset, "Deployment", "kube-system", "coredns", "coredns", extraVars, false)
		})
	})

	// Check TenantControlPlane Konnectivity addon
	It("Should handle Konnectivity extra env var configuration", func() {
		By("creating TCP with default Konnectivity addon configuration", func() {
			konnectivityAddon := &kamajiv1alpha1.KonnectivitySpec{
				KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
					Port: 30132,
				},
			}

			tcp = CreateKindTCPWithAddons("default", "tcp-konnectivity-extra-env-var", kamajiv1alpha1.AddonsSpec{Konnectivity: konnectivityAddon})

			Expect(k8sClient.Create(ctx, tcp)).NotTo(HaveOccurred())
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		})

		var clientset *kubernetes.Clientset

		By("getting TCP clientset", func() {
			clientset, kubeconfigFile = GetTenantClientSet(tcp)
		})

		By("checking env vars for default Konnectivity agent are empty", func() {
			CheckTemplateContainerEnvVars(clientset, "DaemonSet", "kube-system", "konnectivity-agent", "konnectivity-agent", []corev1.EnvVar{}, true)
		})

		By("checking env vars for default Konnectivity server are empty", func() {
			CheckTCPContainerEnvVars(k8sClient, *tcp, "konnectivity-server", []corev1.EnvVar{}, true)
		})

		extraVars := []corev1.EnvVar{{Name: "MY_VAR", Value: "MY_VALUE"}}

		By("adding extra env vars for Konnectivy server", func() {
			updatedTCP := &kamajiv1alpha1.TenantControlPlane{}

			Eventually(func() error {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, updatedTCP)).Should(Succeed())
				updatedTCP.Spec.Addons.Konnectivity = &kamajiv1alpha1.KonnectivitySpec{
					KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
						Port:      30132,
						ExtraEnvs: extraVars,
					},
				}

				return k8sClient.Update(ctx, updatedTCP)
			}).WithTimeout(1 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())

			StatusMustEqualTo(updatedTCP, kamajiv1alpha1.VersionReady)
		})

		By("checking env vars for updated Konnectivity agent are still empty", func() {
			CheckTemplateContainerEnvVars(clientset, "DaemonSet", "kube-system", "konnectivity-agent", "konnectivity-agent", []corev1.EnvVar{}, true)
		})

		By("checking extra env vars for updated Konnectivity server are present", func() {
			CheckTCPContainerEnvVars(k8sClient, *tcp, "konnectivity-server", extraVars, false)
		})

		By("adding extra env vars for Konnectivy agent", func() {
			updatedTCP := &kamajiv1alpha1.TenantControlPlane{}

			Eventually(func() error {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, updatedTCP)).Should(Succeed())
				updatedTCP.Spec.Addons.Konnectivity = &kamajiv1alpha1.KonnectivitySpec{
					KonnectivityAgentSpec: kamajiv1alpha1.KonnectivityAgentSpec{
						ExtraEnvs: extraVars,
					},
					KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
						Port:      30132,
						ExtraEnvs: extraVars,
					},
				}

				return k8sClient.Update(ctx, updatedTCP)
			}).WithTimeout(1 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())

			StatusMustEqualTo(updatedTCP, kamajiv1alpha1.VersionReady)
		})

		By("checking extra env vars for updated Konnectivity agent are present", func() {
			CheckTemplateContainerEnvVars(clientset, "DaemonSet", "kube-system", "konnectivity-agent", "konnectivity-agent", extraVars, false)
		})

		By("checking extra env vars for updated Konnectivity server are still present", func() {
			CheckTCPContainerEnvVars(k8sClient, *tcp, "konnectivity-server", extraVars, false)
		})
	})

	// Check TenantControlPlane KubeProxy addon
	It("Should handle KubeProxy extra env var configuration", func() {
		By("creating TCP with default KubeProxy addon configuration", func() {
			tcp = CreateKindTCPWithAddons("default", "tcp-kubeproxy-extra-env-var", kamajiv1alpha1.AddonsSpec{KubeProxy: &kamajiv1alpha1.AddonSpec{}})

			Expect(k8sClient.Create(ctx, tcp)).NotTo(HaveOccurred())
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		})

		var clientset *kubernetes.Clientset

		By("getting TCP clientset", func() {
			clientset, kubeconfigFile = GetTenantClientSet(tcp)
		})

		defaultVars := []corev1.EnvVar{
			{
				Name: "NODE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "spec.nodeName",
					},
				},
			},
		}

		By("checking env vars for default KubeProxy DaemonSet are set to defaults", func() {
			CheckTemplateContainerEnvVars(clientset, "DaemonSet", "kube-system", "kube-proxy", "kube-proxy", defaultVars, true)
		})

		extraVars := []corev1.EnvVar{{Name: "MY_VAR", Value: "MY_VALUE"}}

		By("adding extra env vars for KubeProxy", func() {
			updatedTCP := &kamajiv1alpha1.TenantControlPlane{}

			Eventually(func() error {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, updatedTCP)).Should(Succeed())
				updatedTCP.Spec.Addons.KubeProxy = &kamajiv1alpha1.AddonSpec{ExtraEnvs: extraVars}

				return k8sClient.Update(ctx, updatedTCP)
			}).WithTimeout(1 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())

			StatusMustEqualTo(updatedTCP, kamajiv1alpha1.VersionReady)
		})

		By("checking extra env vars for updated KubeProxy deployment are present", func() {
			CheckTemplateContainerEnvVars(clientset, "DaemonSet", "kube-system", "kube-proxy", "kube-proxy", extraVars, false)
		})
	})
})
