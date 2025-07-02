// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubeadmv1beta4 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("starting a kind worker with kubeadm", func() {
	ctx := context.Background()

	var tcp *kamajiv1alpha1.TenantControlPlane

	var workerContainer testcontainers.Container

	var kubeconfigFile *os.File

	JustBeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-nodes-join",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: pointer.To(int32(1)),
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
					Version: "v1.29.0",
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

		workerContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				HostConfigModifier: func(config *container.HostConfig) {
					config.Mounts = []mount.Mount{
						{
							Type:   mount.TypeBind,
							Source: "/lib/modules",
							Target: "/lib/modules",
						},
					}
					config.Privileged = true
				},
				Name:     fmt.Sprintf("%s-worker-node", tcp.GetName()),
				Image:    fmt.Sprintf("kindest/node:%s", tcp.Spec.Kubernetes.Version),
				Networks: []string{"kind"},
			},
			Started: true,
		})
		Expect(err).ToNot(HaveOccurred())

		kubeconfigFile, err = os.CreateTemp("", "kamaji")
		Expect(err).ToNot(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(workerContainer.Terminate(ctx)).ToNot(HaveOccurred())
		Expect(k8sClient.Delete(ctx, tcp)).Should(Succeed())
		Expect(os.Remove(kubeconfigFile.Name())).ToNot(HaveOccurred())
	})

	It("should join the Tenant Control Plane cluster", func() {
		By("waiting for the Tenant Control Plane being ready", func() {
			StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		})

		By("downloading Tenant Control Plane kubeconfig", func() {
			secret := &corev1.Secret{}

			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.Status.KubeConfig.Admin.SecretName}, secret)).NotTo(HaveOccurred())

			_, err := kubeconfigFile.Write(secret.Data["admin.conf"])
			Expect(err).ToNot(HaveOccurred())
		})

		var joinCommandBuffer *bytes.Buffer

		By("generating kubeadm join command", func() {
			joinCommandBuffer = bytes.NewBufferString("")

			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile.Name())
			Expect(err).ToNot(HaveOccurred())

			clientset, err := kubernetes.NewForConfig(config)
			Expect(err).ToNot(HaveOccurred())

			// Soot controller might take a while to set up RBAC.
			Eventually(func() error {
				joinCommandBuffer.Reset()

				return cmd.RunCreateToken(joinCommandBuffer, clientset, "", &kubeadmv1beta4.InitConfiguration{}, true, "", kubeconfigFile.Name())
			}, 1*time.Minute, 1*time.Second).Should(Succeed())
		})

		By("enabling br_netfilter", func() {
			exitCode, stdout, err := workerContainer.Exec(ctx, []string{"modprobe", "br_netfilter"})

			out, _ := io.ReadAll(stdout)
			if len(out) > 0 {
				_, _ = fmt.Fprintln(GinkgoWriter, "modprobe failed: "+string(out))
			}

			if exitCode != 0 {
				_, _ = fmt.Fprintln(GinkgoWriter, "modprobe exit code: "+strconv.FormatUint(uint64(exitCode), 10))
			}

			if err != nil {
				_, _ = fmt.Fprintln(GinkgoWriter, "modprobe error: "+err.Error())
			}
		})

		By("disabling swap", func() {
			exitCode, stdout, err := workerContainer.Exec(ctx, []string{"swapoff", "-a"})

			out, _ := io.ReadAll(stdout)
			if len(out) > 0 {
				_, _ = fmt.Fprintln(GinkgoWriter, "swapoff failed: "+string(out))
			}

			if exitCode != 0 {
				_, _ = fmt.Fprintln(GinkgoWriter, "swapoff exit code: "+strconv.FormatUint(uint64(exitCode), 10))
			}

			if err != nil {
				_, _ = fmt.Fprintln(GinkgoWriter, "swapoff error: "+err.Error())
			}
		})

		By("executing the command in the worker node", func() {
			cmds := append(strings.Split(strings.TrimSpace(joinCommandBuffer.String()), " "), "--ignore-preflight-errors=SystemVerification,FileExisting")

			exitCode, stdout, err := workerContainer.Exec(ctx, cmds)

			out, _ := io.ReadAll(stdout)
			if len(out) > 0 {
				_, _ = fmt.Fprintln(GinkgoWriter, "executing failed: "+string(out))
			}

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
