// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	kamajiNamespace   = "kamaji-system"
	kineContainerName = "kine"
	managerContainer  = "manager"
	kineImageArg      = "--kine-image"
)

// kamajiManagerDeployment returns the controller-manager Deployment installed by the e2e setup.
func kamajiManagerDeployment() *appsv1.Deployment {
	GinkgoHelper()

	list := &appsv1.DeploymentList{}
	Expect(k8sClient.List(context.Background(), list,
		ctrlclient.InNamespace(kamajiNamespace),
		ctrlclient.MatchingLabels{"app.kubernetes.io/component": "controller-manager"},
	)).To(Succeed())
	Expect(list.Items).To(HaveLen(1))

	return &list.Items[0]
}

// managerArgs returns a copy of the args of the manager container.
func managerArgs(deployment *appsv1.Deployment) []string {
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == managerContainer {
			return append([]string(nil), c.Args...)
		}
	}

	Fail(fmt.Sprintf("container %q not found in %s", managerContainer, deployment.GetName()))

	return nil
}

// setManagerArgs replaces the args of the manager container, retrying on conflicts with the
// Deployment controller, and returns once the manager rollout has completed.
func setManagerArgs(args []string) {
	GinkgoHelper()

	Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment := kamajiManagerDeployment()

		for i, c := range deployment.Spec.Template.Spec.Containers {
			if c.Name == managerContainer {
				deployment.Spec.Template.Spec.Containers[i].Args = args
			}
		}

		return k8sClient.Update(context.Background(), deployment)
	})).To(Succeed())

	Eventually(func(g Gomega) {
		deployment := kamajiManagerDeployment()
		desired := pointer.Deref(deployment.Spec.Replicas, 1)

		g.Expect(deployment.Status.ObservedGeneration).To(BeNumerically(">=", deployment.GetGeneration()))
		g.Expect(deployment.Status.UpdatedReplicas).To(Equal(desired))
		g.Expect(deployment.Status.AvailableReplicas).To(Equal(desired))
		// Old pods are gone once the total matches the desired count.
		g.Expect(deployment.Status.Replicas).To(Equal(desired))
	}, 5*time.Minute, time.Second).Should(Succeed())

	// The Deployment status does not count pods that are already terminating, but their endpoints stay
	// in the webhook Service until the endpoints controller removes them. Wait until none are left.
	Eventually(func(g Gomega) {
		pods := &corev1.PodList{}
		g.Expect(k8sClient.List(context.Background(), pods,
			ctrlclient.InNamespace(kamajiNamespace),
			ctrlclient.MatchingLabels{"app.kubernetes.io/component": "controller-manager"},
		)).To(Succeed())

		for _, pod := range pods.Items {
			g.Expect(pod.GetDeletionTimestamp()).To(BeNil(), "pod %s is still terminating", pod.GetName())
		}

		slices := &discoveryv1.EndpointSliceList{}
		g.Expect(k8sClient.List(context.Background(), slices,
			ctrlclient.InNamespace(kamajiNamespace),
			ctrlclient.MatchingLabels{discoveryv1.LabelServiceName: "kamaji-webhook-service"},
		)).To(Succeed())

		for _, slice := range slices.Items {
			for _, ep := range slice.Endpoints {
				g.Expect(pointer.Deref(ep.Conditions.Terminating, false)).To(BeFalse(), "endpoint %v is still terminating", ep.Addresses)
			}
		}
	}, 5*time.Minute, time.Second).Should(Succeed())
}

// withKineImageArg returns args with --kine-image set to image, replacing any existing value.
func withKineImageArg(args []string, image string) []string {
	out := make([]string, 0, len(args)+1)

	for _, a := range args {
		if !strings.HasPrefix(a, kineImageArg+"=") && a != kineImageArg {
			out = append(out, a)
		}
	}

	return append(out, kineImageArg+"="+image)
}

// kineRolloutTenantClient returns a client for the tenant control plane API server.
func kineRolloutTenantClient(tcp *kamajiv1alpha1.TenantControlPlane) ctrlclient.Client {
	GinkgoHelper()

	config, err := utilities.GetTenantKubeconfig(context.Background(), k8sClient, tcp)
	Expect(err).ToNot(HaveOccurred())

	b, err := utilities.EncodeToYaml(config)
	Expect(err).ToNot(HaveOccurred())

	clientCfg, err := clientcmd.NewClientConfigFromBytes(b)
	Expect(err).ToNot(HaveOccurred())

	restConfig, err := clientCfg.ClientConfig()
	Expect(err).ToNot(HaveOccurred())

	tcpClient, err := ctrlclient.New(restConfig, ctrlclient.Options{})
	Expect(err).ToNot(HaveOccurred())

	return tcpClient
}

// kineImageOfTenantDeployment returns the kine sidecar image of the tenant control plane Deployment.
func kineImageOfTenantDeployment(tcp *kamajiv1alpha1.TenantControlPlane) string {
	deployment := &appsv1.Deployment{}
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, deployment)).To(Succeed())

	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == kineContainerName {
			return c.Image
		}
	}

	Fail(fmt.Sprintf("container %q not found in Deployment %s", kineContainerName, deployment.GetName()))

	return ""
}

var _ = Describe("Rolling a new default --kine-image out to a live kine TenantControlPlane", Serial, func() {
	var tcp *kamajiv1alpha1.TenantControlPlane
	var upgradeImage string

	BeforeEach(func() {
		// The image is supplied by the environment rather than hardcoded, so the spec does not
		// assume a kine tag exists in the registry the runner can reach.
		upgradeImage = os.Getenv("E2E_KINE_IMAGE_UPGRADE")
		Expect(upgradeImage).ToNot(BeEmpty(), "E2E_KINE_IMAGE_UPGRADE must name a kine image different from the manager default")
	})

	// Create a kine-backed TenantControlPlane and wait for it to be Ready
	JustBeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("kine-rollout-%s", rand.String(5)),
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				DataStore: "mysql-bronze",
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
					Port:    int32(rand.Int63nRange(31000, 32000)),
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: kamajiv1alpha1.DefaultKubernetesVersion,
					Kubelet: kamajiv1alpha1.KubeletSpec{
						ConfigurationJSONPatches: kamajiv1alpha1.JSONPatches{
							{
								Op:   "replace",
								Path: "/cgroupDriver",
								Value: &apiextensionsv1.JSON{
									Raw: []byte(`"cgroupfs"`),
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
	})

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	It("moves the tenant to the new default image and keeps its data", func() {
		By("writing data through the tenant API server before the rollout")
		tenantClient := kineRolloutTenantClient(tcp)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kine-rollout-data",
				Namespace: "default",
			},
			Data: map[string]string{"value": rand.String(16)},
		}
		Expect(tenantClient.Create(context.Background(), cm)).To(Succeed())

		By("recording the current kine image of the tenant")
		previousImage := kineImageOfTenantDeployment(tcp)
		Expect(previousImage).ToNot(Equal(upgradeImage), "E2E_KINE_IMAGE_UPGRADE must differ from the image the tenant already runs")

		By("setting the new default --kine-image on the controller-manager")
		original := managerArgs(kamajiManagerDeployment())
		DeferCleanup(func() {
			setManagerArgs(original)
		})

		setManagerArgs(withKineImageArg(original, upgradeImage))

		By("waiting for the tenant Deployment to adopt the new kine image")
		Eventually(func() string {
			return kineImageOfTenantDeployment(tcp)
		}, 5*time.Minute, time.Second).Should(Equal(upgradeImage))

		By("waiting for the tenant control plane to return to Ready")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, deployment)).To(Succeed())
			desired := pointer.Deref(deployment.Spec.Replicas, 1)
			g.Expect(deployment.Status.UpdatedReplicas).To(Equal(desired))
			g.Expect(deployment.Status.ReadyReplicas).To(Equal(desired))
			g.Expect(deployment.Status.Replicas).To(Equal(desired))
		}, 5*time.Minute, time.Second).Should(Succeed())

		// The Deployment status does not count pods that are already terminating, and the old tenant
		// API server keeps its NodePort endpoint until it has gone. Wait until no tenant pod is left.
		Eventually(func(g Gomega) {
			pods, err := getControlPlanePods(tcp)
			g.Expect(err).ToNot(HaveOccurred())

			for _, pod := range pods.Items {
				g.Expect(pod.GetDeletionTimestamp()).To(BeNil(), "pod %s is still terminating", pod.GetName())
			}
		}, 5*time.Minute, time.Second).Should(Succeed())
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

		By("checking the data written before the rollout is still readable")
		got := &corev1.ConfigMap{}
		Expect(kineRolloutTenantClient(tcp).Get(context.Background(), ctrlclient.ObjectKeyFromObject(cm), got)).To(Succeed())
		Expect(got.Data).To(Equal(cm.Data))
	})
})
