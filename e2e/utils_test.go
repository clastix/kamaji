// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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

	kubectlExec := func(ctx context.Context, args ...string) {
		cmd := exec.CommandContext(ctx, "kubectl")

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
			context.Background(),
			"--namespace="+tcp.GetNamespace(),
			"get",
			"tcp",
			tcp.GetName(),
		)
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane resources")
		kubectlExec(
			context.Background(),
			"--namespace="+tcp.GetNamespace(),
			"get",
			"svc,deployment,pods,ep,configmap,secrets",
		)
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane pods")
		kubectlExec(
			context.Background(),
			"--namespace="+tcp.GetNamespace(),
			"describe",
			"pods",
		)
	}
}

// kamajiTimeline records, for the whole suite, the changes to the controller-manager Pods and to the
// EndpointSlices of kamaji-webhook-service, so a failing spec can be matched against them.
var (
	kamajiTimelineMu sync.Mutex
	kamajiTimeline   []string
)

func recordKamajiTimelinef(format string, args ...any) {
	kamajiTimelineMu.Lock()
	defer kamajiTimelineMu.Unlock()

	kamajiTimeline = append(kamajiTimeline, time.Now().Format(time.RFC3339Nano)+" "+fmt.Sprintf(format, args...))
}

func boolPtrString(b *bool) string {
	if b == nil {
		return "nil"
	}

	return fmt.Sprintf("%t", *b)
}

// startKamajiTimelineRecorder starts informers that record the controller-manager Pods and the
// webhook EndpointSlices. It runs until the test process exits.
func startKamajiTimelineRecorder() {
	GinkgoHelper()

	clientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	podFactory := informers.NewSharedInformerFactoryWithOptions(clientset, 0,
		informers.WithNamespace(kamajiNamespace),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.LabelSelector = "app.kubernetes.io/component=controller-manager"
		}))
	_, err = podFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { recordPodChange("added", obj) },
		UpdateFunc: func(_, obj interface{}) { recordPodChange("updated", obj) },
		DeleteFunc: func(obj interface{}) { recordPodChange("deleted", obj) },
	})
	Expect(err).ToNot(HaveOccurred())

	sliceFactory := informers.NewSharedInformerFactoryWithOptions(clientset, 0,
		informers.WithNamespace(kamajiNamespace),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.LabelSelector = discoveryv1.LabelServiceName + "=kamaji-webhook-service"
		}))
	_, err = sliceFactory.Discovery().V1().EndpointSlices().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { recordSliceChange("added", obj) },
		UpdateFunc: func(_, obj interface{}) { recordSliceChange("updated", obj) },
		DeleteFunc: func(obj interface{}) { recordSliceChange("deleted", obj) },
	})
	Expect(err).ToNot(HaveOccurred())

	// Tenant control planes live in default, and their NodePort services are what the specs connect to.
	tenantSliceFactory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace("default"))
	_, err = tenantSliceFactory.Discovery().V1().EndpointSlices().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { recordSliceChange("added", obj) },
		UpdateFunc: func(_, obj interface{}) { recordSliceChange("updated", obj) },
		DeleteFunc: func(obj interface{}) { recordSliceChange("deleted", obj) },
	})
	Expect(err).ToNot(HaveOccurred())

	stop := make(chan struct{})
	podFactory.Start(stop)
	sliceFactory.Start(stop)
	tenantSliceFactory.Start(stop)
}

func recordPodChange(action string, obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		recordKamajiTimelinef("pod %s: unexpected object %T", action, obj)

		return
	}

	recordKamajiTimelinef("pod %s %s phase=%s ready=%t deleting=%t startTime=%v",
		action, pod.GetName(), pod.Status.Phase, isPodReady(pod), pod.GetDeletionTimestamp() != nil, pod.Status.StartTime)
}

func recordSliceChange(action string, obj interface{}) {
	slice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		recordKamajiTimelinef("endpointslice %s: unexpected object %T", action, obj)

		return
	}

	if len(slice.Endpoints) == 0 {
		recordKamajiTimelinef("endpointslice %s %s: no endpoints", action, slice.GetName())

		return
	}

	for _, ep := range slice.Endpoints {
		recordKamajiTimelinef("endpointslice %s %s addresses=%v ready=%s serving=%s terminating=%s target=%s",
			action, slice.GetName(), ep.Addresses,
			boolPtrString(ep.Conditions.Ready), boolPtrString(ep.Conditions.Serving), boolPtrString(ep.Conditions.Terminating),
			targetName(ep.TargetRef))
	}
}

// PrintKamajiLogs dumps the state of the Kamaji controller-manager (which also serves the webhooks)
// when a spec fails: the recorded timeline, every controller-manager Pod, its current and previous
// logs, the webhook Service endpoints and the events in the Kamaji namespace.
func PrintKamajiLogs() {
	if !CurrentSpecReport().Failed() {
		return
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	ctx := context.Background()

	_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Kamaji timeline, controller-manager Pods and webhook EndpointSlices")
	kamajiTimelineMu.Lock()
	for _, line := range kamajiTimeline {
		_, _ = fmt.Fprintln(GinkgoWriter, ">>> ", line)
	}
	kamajiTimelineMu.Unlock()
	_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: end of Kamaji timeline")

	// The window covers the spec that ran before this one, which may restart the controller-manager.
	sinceSeconds := int64((5 * time.Minute).Seconds())

	_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Kamaji controller-manager Pods")
	pods, err := clientset.CoreV1().Pods(kamajiNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=controller-manager",
	})
	Expect(err).ToNot(HaveOccurred())

	for _, pod := range pods.Items {
		_, _ = fmt.Fprintf(GinkgoWriter, ">>> pod=%s phase=%s created=%s deleting=%t\n",
			pod.GetName(), pod.Status.Phase, pod.GetCreationTimestamp().Format(time.RFC3339), pod.GetDeletionTimestamp() != nil)

		for _, cs := range pod.Status.ContainerStatuses {
			_, _ = fmt.Fprintf(GinkgoWriter, ">>> container=%s ready=%t restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
		}

		for _, cond := range pod.Status.Conditions {
			_, _ = fmt.Fprintf(GinkgoWriter, ">>> condition=%s status=%s reason=%s\n", cond.Type, cond.Status, cond.Reason)
		}

		printPodLogs(clientset, pod.GetName(), false, sinceSeconds)
		printPodLogs(clientset, pod.GetName(), true, sinceSeconds)
	}

	_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: kamaji-webhook-service Endpoints")
	endpoints, err := clientset.CoreV1().Endpoints(kamajiNamespace).Get(ctx, "kamaji-webhook-service", metav1.GetOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, ">>> error: %v\n", err)
	} else {
		for _, subset := range endpoints.Subsets {
			for _, addr := range subset.Addresses {
				_, _ = fmt.Fprintf(GinkgoWriter, ">>> ready address=%s target=%s\n", addr.IP, targetName(addr.TargetRef))
			}

			for _, addr := range subset.NotReadyAddresses {
				_, _ = fmt.Fprintf(GinkgoWriter, ">>> notReady address=%s target=%s\n", addr.IP, targetName(addr.TargetRef))
			}
		}
	}

	_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Kamaji events, oldest first")
	events, err := clientset.CoreV1().Events(kamajiNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, ">>> error: %v\n", err)
	} else {
		sort.Slice(events.Items, func(i, j int) bool {
			return events.Items[i].LastTimestamp.Time.Before(events.Items[j].LastTimestamp.Time)
		})

		for _, ev := range events.Items {
			_, _ = fmt.Fprintf(GinkgoWriter, ">>> %s %s %s/%s reason=%s: %s\n",
				ev.LastTimestamp.Format(time.RFC3339), ev.Type, ev.InvolvedObject.Kind, ev.InvolvedObject.Name, ev.Reason, ev.Message)
		}
	}
}

// printPodLogs prints the logs of the manager container of a controller-manager Pod. Errors are
// printed rather than failing, because the previous container may not exist.
func printPodLogs(clientset kubernetes.Interface, podName string, previous bool, sinceSeconds int64) {
	kind := "current"
	if previous {
		kind = "previous"
	}

	_, _ = fmt.Fprintf(GinkgoWriter, "DEBUG: %s logs of Pod %s\n", kind, podName)

	stream, err := clientset.CoreV1().Pods(kamajiNamespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:    managerContainer,
		Previous:     previous,
		SinceSeconds: &sinceSeconds,
		Timestamps:   true,
	}).Stream(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, ">>> error: %v\n", err)

		return
	}

	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, ">>> error: %v\n", err)

		return
	}

	for _, line := range bytes.Split(body, []byte("\n")) {
		_, _ = fmt.Fprintln(GinkgoWriter, ">>> ", string(line))
	}

	_, _ = fmt.Fprintf(GinkgoWriter, "DEBUG: end of %s logs of Pod %s\n", kind, podName)
}

func targetName(ref *corev1.ObjectReference) string {
	if ref == nil {
		return ""
	}

	return ref.Name
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
		tcpPods, err := getControlPlanePods(tcp)
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
		tcpPods, err := getControlPlanePods(tcp)
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
		tcpPods, err := getControlPlanePods(tcp)
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
	tcpPods, err := getControlPlanePods(tcp)
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
	tcpPods, err := getControlPlanePods(tcp)
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
