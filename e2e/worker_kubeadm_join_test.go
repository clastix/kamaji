// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/util"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("starting a kind worker with kubeadm", func() {
	ctx := context.Background()

	var tcp kamajiv1alpha1.TenantControlPlane

	var workerContainer testcontainers.Container

	var kubeconfigFile *os.File

	JustBeforeEach(func() {
		// Retrieving the kind instance IP from the `kubernetes` service in the `default` namespace
		ep := &corev1.Endpoints{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, ep)).ToNot(HaveOccurred())

		tcp = kamajiv1alpha1.TenantControlPlane{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-nodes-join",
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
					Address: ep.Subsets[0].Addresses[0].IP,
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
		Expect(k8sClient.Create(ctx, &tcp)).NotTo(HaveOccurred())

		var err error

		workerContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name:       fmt.Sprintf("%s-worker-node", tcp.GetName()),
				Image:      fmt.Sprintf("kindest/node:%s", tcp.Spec.Kubernetes.Version),
				Mounts:     testcontainers.ContainerMounts{testcontainers.BindMount("/lib/modules", "/lib/modules")},
				Networks:   []string{"kind"},
				Privileged: true,
			},
			Started: true,
		})
		Expect(err).ToNot(HaveOccurred())

		kubeconfigFile, err = ioutil.TempFile("", "kamaji")
		Expect(err).ToNot(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(ctx, &tcp)).Should(Succeed())
		Expect(workerContainer.Terminate(ctx)).ToNot(HaveOccurred())
		Expect(os.Remove(kubeconfigFile.Name())).ToNot(HaveOccurred())
	})

	It("should join the Tenant Control Plane cluster", func() {
		By("waiting for the Tenant Control Plane being ready", func() {
			Eventually(func() kamajiv1alpha1.KubernetesVersionStatus {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, &tcp)
				if err != nil {
					return ""
				}

				if tcp.Status.Kubernetes.Version.Status == nil {
					return ""
				}

				return *tcp.Status.Kubernetes.Version.Status
			}, 5*time.Minute, time.Second).Should(Equal(kamajiv1alpha1.VersionReady))
		})

		By("downloading Tenant Control Plane kubeconfig", func() {
			secret := &corev1.Secret{}

			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.Status.KubeConfig.Admin.SecretName}, secret)).NotTo(HaveOccurred())

			_, err := kubeconfigFile.Write(secret.Data["admin.conf"])
			Expect(err).ToNot(HaveOccurred())
		})

		var joinCommandBuffer *bytes.Buffer

		By("generating kubeadm join command", func() {
			joinCommandBuffer = bytes.NewBuffer([]byte(""))

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile.Name())
			Expect(err).ToNot(HaveOccurred())

			clientset, err := kubernetes.NewForConfig(config)
			Expect(err).ToNot(HaveOccurred())

			Expect(cmd.RunCreateToken(joinCommandBuffer, clientset, "", util.DefaultInitConfiguration(), true, "", kubeconfigFile.Name())).ToNot(HaveOccurred())
		})

		By("executing the command in the worker node", func() {
			cmds := append(strings.Split(strings.TrimSpace(joinCommandBuffer.String()), " "), "--ignore-preflight-errors", "SystemVerification")

			exitCode, err := workerContainer.Exec(ctx, cmds)
			Expect(exitCode).To(Equal(0))
			Expect(err).ToNot(HaveOccurred())
		})

		By("waiting for nodes", func() {
			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile.Name())
			Expect(err).ToNot(HaveOccurred())

			clientset, err := kubernetes.NewForConfig(config)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				if err != nil {
					return ""
				}

				if len(nodes.Items) == 0 {
					return ""
				}

				return nodes.Items[0].GetName()
			}, time.Minute, time.Second).Should(Equal(workerContainer.GetContainerID()[:12]))
		})
	})
})
