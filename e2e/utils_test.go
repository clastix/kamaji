// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func GetKindIPAddress() string {
	var ep discoveryv1.EndpointSlice
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "kubernetes", Namespace: "default"}, &ep)).ToNot(HaveOccurred())

	return ep.Endpoints[0].Addresses[0]
}

func PrintTenantControlPlaneInfo() {
	tcpList := &kamajiv1alpha1.TenantControlPlaneList{}
	Expect(k8sClient.List(context.Background(), tcpList)).ToNot(HaveOccurred())

	if len(tcpList.Items) == 0 {
		return
	}

	tcp := tcpList.Items[0]

	kubectlExec := func(args ...string) {
		cmd := exec.Command("kubectl")

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Args = args

		Expect(cmd.Run()).ToNot(HaveOccurred())

		for {
			line, err := out.ReadString('\n')
			if err != nil {
				return
			}

			_, _ = fmt.Fprint(GinkgoWriter, ">>> ", line)
		}
	}

	if CurrentSpecReport().Failed() {
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane definition")
		kubectlExec(
			fmt.Sprintf("--namespace=%s", tcp.GetNamespace()),
			"get",
			"tcp",
			tcp.GetName(),
		)
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane resources")
		kubectlExec(
			fmt.Sprintf("--namespace=%s", tcp.GetNamespace()),
			"get",
			"svc,deployment,pods,ep,configmap,secrets",
		)
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane pods")
		kubectlExec(
			fmt.Sprintf("--namespace=%s", tcp.GetNamespace()),
			"describe",
			"pods",
		)
	}
}

func PrintKamajiLogs() {
	if CurrentSpecReport().Failed() {
		clientset, err := kubernetes.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())

		list, err := clientset.CoreV1().Pods("kamaji-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/component=controller-manager",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(list.Items).To(HaveLen(1))

		request := clientset.CoreV1().Pods("kamaji-system").GetLogs(list.Items[0].GetName(), &corev1.PodLogOptions{
			Container: "manager",
			SinceSeconds: func() *int64 {
				seconds := int64(CurrentSpecReport().RunTime)

				return &seconds
			}(),
			Timestamps: true,
		})

		podLogs, err := request.Stream(context.Background())
		Expect(err).ToNot(HaveOccurred())

		defer podLogs.Close()

		podBytes, err := io.ReadAll(podLogs)
		Expect(err).ToNot(HaveOccurred())

		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: retrieving Kamaji Pod logs")

		for _, line := range bytes.Split(podBytes, []byte("\n")) {
			_, _ = fmt.Fprintln(GinkgoWriter, ">>> ", string(line))
		}

		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: end of Kamaji Pod logs")
	}
}

func StatusMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, status kamajiv1alpha1.KubernetesVersionStatus) {
	GinkgoHelper()
	Eventually(func() kamajiv1alpha1.KubernetesVersionStatus {
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), tcp)
		if err != nil {
			return ""
		}
		// Check if Status field has been created on TenantControlPlane struct
		if tcp.Status.Kubernetes.Version.Status == nil {
			return ""
		}

		return *tcp.Status.Kubernetes.Version.Status
	}, 5*time.Minute, time.Second).Should(Equal(status))
}

func AllPodsLabelMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, label string, value string) {
	GinkgoHelper()
	Eventually(func() bool {
		tcpPods := &corev1.PodList{}
		err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
			"kamaji.clastix.io/name": tcp.GetName(),
		})
		if err != nil {
			return false
		}
		for _, pod := range tcpPods.Items {
			if pod.Labels[label] != value {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

func AllPodsAnnotationMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, annotation string, value string) {
	GinkgoHelper()
	Eventually(func() bool {
		tcpPods := &corev1.PodList{}
		err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
			"kamaji.clastix.io/name": tcp.GetName(),
		})
		if err != nil {
			return false
		}
		for _, pod := range tcpPods.Items {
			if pod.Annotations[annotation] != value {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

func PodsServiceAccountMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, sa *corev1.ServiceAccount) {
	GinkgoHelper()
	saName := sa.GetName()
	Eventually(func() bool {
		tcpPods := &corev1.PodList{}
		err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
			"kamaji.clastix.io/name": tcp.GetName(),
		})
		if err != nil {
			return false
		}
		for _, pod := range tcpPods.Items {
			if pod.Spec.ServiceAccountName != saName {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

func ScaleTenantControlPlane(tcp *kamajiv1alpha1.TenantControlPlane, replicas int32) {
	GinkgoHelper()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), tcp)).To(Succeed())
		tcp.Spec.ControlPlane.Deployment.Replicas = &replicas

		return k8sClient.Update(context.Background(), tcp)
	})
	Expect(err).To(Succeed())
}

// CreateGatewayWithListeners creates a Gateway with control plane and konnectivity-server listeners.
func CreateGatewayWithListeners(gatewayName, namespace, gatewayClassName, hostname string) {
	GinkgoHelper()
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: namespace,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(gatewayClassName),
			Listeners: []gatewayv1.Listener{
				{
					Name:     "cp-listener",
					Port:     6443,
					Protocol: gatewayv1.TLSProtocolType,
					Hostname: pointer.To(gatewayv1.Hostname(hostname)),
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: pointer.To(gatewayv1.TLSModeType("Passthrough")),
					},
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: pointer.To(gatewayv1.NamespacesFromAll),
						},
						Kinds: []gatewayv1.RouteGroupKind{
							{
								Group: pointer.To(gatewayv1.Group("gateway.networking.k8s.io")),
								Kind:  "TLSRoute",
							},
						},
					},
				},
				{
					Name:     "konnectivity-server",
					Port:     8132,
					Protocol: gatewayv1.TLSProtocolType,
					Hostname: pointer.To(gatewayv1.Hostname(hostname)),
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: pointer.To(gatewayv1.TLSModeType("Passthrough")),
					},
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: pointer.To(gatewayv1.NamespacesFromAll),
						},
						Kinds: []gatewayv1.RouteGroupKind{
							{
								Group: pointer.To(gatewayv1.Group("gateway.networking.k8s.io")),
								Kind:  "TLSRoute",
							},
						},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), gateway)).NotTo(HaveOccurred())
}

// containerSecurityContextMustEqualTo verifies if the container with the given containerName in the control plane pods has the given security context.
func containerSecurityContextMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, containerName string, containerSecurityContext *corev1.SecurityContext) {
	GinkgoHelper()
	tcpPods := &corev1.PodList{}
	err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
		"kamaji.clastix.io/name": tcp.GetName(),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(tcpPods.Items).ToNot(BeEmpty())

	for _, pod := range tcpPods.Items {
		// containerFound tracks if the container with the given containerName is actually present
		containerFound := false
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				containerFound = true
				Expect(container.SecurityContext).To(Equal(containerSecurityContext), fmt.Sprintf("securityContext for container %s does not match expected value", containerName))
			} else {
				continue
			}
		}
		Expect(containerFound).To(BeTrue(), fmt.Sprintf("pod does not container a container with name '%s'", containerName))
	}
}

// podSecurityContextMustEqualTo verifies if the control plane pods have the given security context.
func podSecurityContextMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, podSecurityContext *corev1.PodSecurityContext) {
	GinkgoHelper()
	tcpPods := &corev1.PodList{}
	err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
		"kamaji.clastix.io/name": tcp.GetName(),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(tcpPods.Items).ToNot(BeEmpty())

	for _, pod := range tcpPods.Items {
		Expect(pod.Spec.SecurityContext).To(Equal(podSecurityContext), "podSecurityContext does not match expected value")
	}
}

// waitUntilControlPlaneReconciliationIsFinished waits until the controller has finished reconciling the control plane deployment.
// This is useful as the controller updates the control plane deployment several times, resulting in multiple replicasets running in parallel.
func waitUntilControlPlaneReconciliationIsFinished(tcp *kamajiv1alpha1.TenantControlPlane) {
	Eventually(func() bool {
		tcpPods, err := getControlPlanePods(tcp)
		if err != nil {
			return false
		}

		// if there are no pods, reconciliation is clearly not finished
		podCount := len(tcpPods.Items)
		if podCount == 0 {
			return false
		}

		// all pods must have the same pod-template-hash and be ready
		firstPodTemplateHash := tcpPods.Items[0].Labels["pod-template-hash"]
		for _, pod := range tcpPods.Items {
			if pod.Labels["pod-template-hash"] != firstPodTemplateHash {
				return false
			}
			if !isPodReady(&pod) {
				return false
			}
		}

		// things are looking good at this point
		// wait an arbitrary amount of time to see if something is still happening
		time.Sleep(5 * time.Second)

		// repeat the process
		tcpPods, err = getControlPlanePods(tcp)
		if err != nil {
			return false
		}

		// the number of pods must stay stable
		if len(tcpPods.Items) != podCount {
			return false
		}

		// verify that the pod template hash is stable and all pods are still ready
		for _, pod := range tcpPods.Items {
			if pod.Labels["pod-template-hash"] != firstPodTemplateHash {
				return false
			}
			if !isPodReady(&pod) {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

// getControlPlanePods returns all pods that belong to the given tenant control plane.
func getControlPlanePods(tcp *kamajiv1alpha1.TenantControlPlane) (*corev1.PodList, error) {
	tcpPods := &corev1.PodList{}
	err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
		"kamaji.clastix.io/name": tcp.GetName(),
	})
	if err != nil {
		return &corev1.PodList{}, err
	}

	return tcpPods, nil
}

// isPodReady returns true if and only if the given Pod has status "Ready" = true.
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
