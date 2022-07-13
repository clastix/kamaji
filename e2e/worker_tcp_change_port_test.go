// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("validating kubeconfig", func() {
	ctx := context.Background()

	var tcp *kamajiv1alpha1.TenantControlPlane

	var kubeconfigFile *os.File

	JustBeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeconfig",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: 1,
					},
					Ingress: kamajiv1alpha1.IngressSpec{
						Enabled: false,
					},
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: "NodePort",
					},
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					Address: GetKindIPAddress(),
					Port:    31443,
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
		Expect(k8sClient.Create(ctx, tcp)).NotTo(HaveOccurred())

		var err error

		kubeconfigFile, err = ioutil.TempFile("", "kamaji")
		Expect(err).ToNot(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(ctx, tcp)).Should(Succeed())
		Expect(os.Remove(kubeconfigFile.Name())).ToNot(HaveOccurred())
	})

	It("return kubernetes version", func() {
		for _, port := range []int32{31444, 31445, 31446} {
			Eventually(func() string {
				By(fmt.Sprintf("ensuring TCP port is set to %d", port), func() {
					Eventually(func() (err error) {
						if err = k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
							_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve TCP:", err.Error())

							return err
						}

						tcp.Spec.NetworkProfile.Port = port

						return k8sClient.Update(ctx, tcp)
					}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
				})

				By("ensuring port change is defined in the TCP status", func() {
					Eventually(func() int32 {
						if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
							_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve TCP:", err.Error())

							return 0
						}

						return tcp.Status.Kubernetes.Service.Port
					}, time.Minute, 5*time.Second).Should(Equal(port))
				})

				By("ensuring downloading the updated kubeconfig", func() {
					Eventually(func() (err error) {
						if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
							_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve TCP:", err.Error())

							return err
						}

						secret := &corev1.Secret{}

						if err = k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.Status.KubeConfig.Admin.SecretName}, secret); err != nil {
							_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve kubeconfig secret name:", err.Error())

							return err
						}

						_, err = kubeconfigFile.Write(secret.Data["admin.conf"])

						return err
					}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
				})

				var version version.Info

				By("retrieving TCP version using the kubeconfig", func() {
					config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile.Name())
					if err != nil {
						_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot generate REST configuration:", err.Error())

						return
					}

					clientset, err := kubernetes.NewForConfig(config)
					if err != nil {
						_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot generate clientset:", err.Error())

						return
					}

					serverVersion, err := clientset.ServerVersion()
					if err != nil {
						_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve server version:", err.Error())

						return
					}

					version = *serverVersion
				})

				return version.GitVersion
			}, 5*time.Minute, 5*time.Second).Should(Equal(tcp.Spec.Kubernetes.Version))
		}
	})
})
