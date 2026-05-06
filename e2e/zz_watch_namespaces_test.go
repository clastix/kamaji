// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

// This spec mutates the running kamaji controller-manager Deployment to assert
// that --watch-namespaces correctly scopes the cache. It is named with the
// `zz_` prefix so Ginkgo executes it after every other spec in the suite,
// minimising the blast radius if the operator restore step were to misbehave.
// Every state change registers a DeferCleanup immediately after capture so the
// original Deployment args are restored even when an assertion aborts the run.

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// isWebhookUnreachable returns true when err describes a transient inability
// to reach the kamaji validating/mutating webhook service. The kube-apiserver
// surfaces these as 500 InternalError with a "failed calling webhook" prefix;
// past that prefix we treat any common network-layer or post-rollout warm-up
// signature as retriable. Anything else (validation errors, conflicts, etc.)
// falls through and aborts the spec.
func isWebhookUnreachable(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	if !strings.Contains(msg, "failed calling webhook") {
		return false
	}

	switch {
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "EOF"),
		strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "no endpoints available"),
		strings.Contains(msg, "no route to host"),
		strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "service unavailable"),
		strings.Contains(msg, "tls: handshake failure"),
		strings.Contains(msg, "x509:"),
		strings.Contains(msg, "not found"):
		return true
	}

	return false
}

// Ordered guarantees in-suite ordering; Serial additionally prevents this
// describe from running in parallel with any other one (e.g. if the suite
// is later invoked with `ginkgo -p`). Both are required because we mutate
// the shared kamaji controller-manager Deployment in BeforeAll.
var _ = Describe("--watch-namespaces", Ordered, Serial, func() {
	const (
		kamajiNamespace      = "kamaji-system"
		controllerLabelKey   = "app.kubernetes.io/component"
		controllerLabelValue = "controller-manager"
		managerContainerName = "manager"
		watchedNs            = "watch-ns-allowed"
		unwatchedNs          = "watch-ns-blocked"
		rolloutTimeout       = 90 * time.Second
		reconcileWindow      = 2 * time.Minute
		ignoreWindow         = 60 * time.Second
	)

	var (
		deploymentKey types.NamespacedName
		originalArgs  []string
	)

	// The kubeconfig-generator Deployment shares the exact same selector
	// labels as the controller-manager (both use kamaji.selectorLabels in
	// the chart), so we further filter by the container name to
	// disambiguate. The chart pins the controller container to "manager"
	// and the kubeconfig-generator container to "controller", which is
	// stable across `extraArgs` rendering changes.
	findManagerDeployment := func() *appsv1.Deployment {
		GinkgoHelper()

		list := &appsv1.DeploymentList{}
		Expect(k8sClient.List(context.Background(), list,
			client.InNamespace(kamajiNamespace),
			client.MatchingLabels{controllerLabelKey: controllerLabelValue},
		)).To(Succeed())

		var matches []*appsv1.Deployment

		for i := range list.Items {
			d := &list.Items[i]
			for _, c := range d.Spec.Template.Spec.Containers {
				if c.Name == managerContainerName {
					matches = append(matches, d)

					break
				}
			}
		}

		Expect(matches).To(HaveLen(1), "expected exactly one kamaji controller-manager Deployment in %s (container name=%q)", kamajiNamespace, managerContainerName)

		return matches[0]
	}

	// waitForRollout waits until the Deployment status is fully rolled out AND
	// only the new generation's pods remain. The standard Deployment-status
	// gate (UpdatedReplicas / AvailableReplicas) flips before the old
	// replica-set's pods finish terminating, which leaves PrintKamajiLogs in
	// utils_test.go observing two pods (its HaveLen(1) assertion fails) and
	// briefly leaves the webhook Service Endpoints with a stale address. The
	// strict pod-count check (len(items) == replicas) covers both: terminating
	// pods are still listed until kubelet finishes their grace period, and
	// only that strict count guarantees a clean steady state.
	waitForRollout := func() {
		GinkgoHelper()

		Eventually(func(g Gomega) {
			d := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(context.Background(), deploymentKey, d)).To(Succeed())
			g.Expect(d.Status.ObservedGeneration).To(BeNumerically(">=", d.Generation))
			g.Expect(d.Status.UpdatedReplicas).To(Equal(d.Status.Replicas))
			g.Expect(d.Status.AvailableReplicas).To(Equal(d.Status.Replicas))

			want := int32(1)
			if d.Spec.Replicas != nil {
				want = *d.Spec.Replicas
			}

			pods := &corev1.PodList{}
			g.Expect(k8sClient.List(context.Background(), pods,
				client.InNamespace(kamajiNamespace),
				client.MatchingLabels{controllerLabelKey: controllerLabelValue},
			)).To(Succeed())

			g.Expect(pods.Items).To(HaveLen(int(want)), "expected exactly %d controller-manager pod(s) and no leftover terminating ones", want)

			for _, p := range pods.Items {
				g.Expect(p.DeletionTimestamp).To(BeNil(), "pod %s/%s is still terminating", p.Namespace, p.Name)
				g.Expect(p.Status.Phase).To(Equal(corev1.PodRunning), "pod %s/%s phase=%s", p.Namespace, p.Name, p.Status.Phase)
			}
		}, rolloutTimeout, 2*time.Second).Should(Succeed())
	}

	// waitForWebhookReady polls the kamaji-webhook-service EndpointSlices
	// until at least one endpoint is Ready. The container's readiness probe
	// targets the manager's healthz port (8081), not the webhook port (9443),
	// so EndpointSlice readiness only proves that the pod is up — not that
	// the webhook server has finished binding 9443 or that kube-proxy has
	// finished syncing the new ClusterIP backend. The actual webhook reach
	// is exercised by retryUntilWebhookReady around each Create call.
	waitForWebhookReady := func() {
		GinkgoHelper()

		Eventually(func(g Gomega) {
			slices := &discoveryv1.EndpointSliceList{}
			g.Expect(k8sClient.List(context.Background(), slices,
				client.InNamespace(kamajiNamespace),
				client.MatchingLabels{discoveryv1.LabelServiceName: "kamaji-webhook-service"},
			)).To(Succeed())

			ready := 0

			for _, s := range slices.Items {
				for _, ep := range s.Endpoints {
					if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
						ready += len(ep.Addresses)
					}
				}
			}

			g.Expect(ready).To(BeNumerically(">=", 1), "kamaji-webhook-service has no Ready endpoints yet")
		}, 60*time.Second, time.Second).Should(Succeed())
	}

	setManagerArgs := func(args []string) {
		GinkgoHelper()

		Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
			d := &appsv1.Deployment{}
			if err := k8sClient.Get(context.Background(), deploymentKey, d); err != nil {
				return err
			}

			d.Spec.Template.Spec.Containers[0].Args = append([]string(nil), args...)

			return k8sClient.Update(context.Background(), d)
		})).To(Succeed())

		waitForRollout()
		waitForWebhookReady()
	}

	// createTCPWithRetry calls k8sClient.Create and retries while the kamaji
	// validating/mutating webhook is unreachable. After a Deployment rollout
	// the webhook Service routes can briefly serve "connection refused" — the
	// pod's readiness probe targets healthz (8081) and flips green before the
	// webhook server (9443) has finished binding, and kube-proxy may also
	// lag by one informer tick. Retrying with a tight backoff makes the test
	// independent of those timing details. Any other error fails immediately.
	createTCPWithRetry := func(tcp *kamajiv1alpha1.TenantControlPlane) {
		GinkgoHelper()

		Eventually(func(g Gomega) {
			err := k8sClient.Create(context.Background(), tcp)
			if err == nil || apierrors.IsAlreadyExists(err) {
				return
			}

			if isWebhookUnreachable(err) {
				g.Expect(err).NotTo(HaveOccurred(), "webhook still unreachable, retrying")

				return
			}

			Fail(fmt.Sprintf("unexpected non-retriable error from Create: %v", err))
		}, 60*time.Second, time.Second).Should(Succeed())
	}

	createNamespace := func(name string) {
		GinkgoHelper()

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err := k8sClient.Create(context.Background(), ns); err != nil && !apierrors.IsAlreadyExists(err) {
			Fail(err.Error())
		}
	}

	tcpFor := func(name, namespace string) *kamajiv1alpha1.TenantControlPlane {
		return &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{Replicas: pointer.To(int32(1))},
					Service:    kamajiv1alpha1.ServiceSpec{ServiceType: "ClusterIP"},
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{Address: "172.18.0.2"},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: "v1.23.6",
					Kubelet: kamajiv1alpha1.KubeletSpec{CGroupFS: "cgroupfs"},
				},
			},
		}
	}

	statusOf := func(tcp *kamajiv1alpha1.TenantControlPlane) *kamajiv1alpha1.KubernetesVersionStatus {
		got := &kamajiv1alpha1.TenantControlPlane{}
		if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), got); err != nil {
			return nil
		}

		return got.Status.Kubernetes.Version.Status
	}

	BeforeAll(func() {
		By("snapshotting the kamaji controller-manager Deployment")
		d := findManagerDeployment()
		deploymentKey = client.ObjectKeyFromObject(d)
		originalArgs = append([]string(nil), d.Spec.Template.Spec.Containers[0].Args...)
		DeferCleanup(func() {
			By("restoring the kamaji controller-manager Deployment args")
			setManagerArgs(originalArgs)
		})

		By("creating the test namespaces")
		for _, name := range []string{watchedNs, unwatchedNs} {
			createNamespace(name)
			DeferCleanup(func(target string) func() {
				return func() {
					By("deleting test namespace " + target)
					_ = k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: target}})
				}
			}(name))
		}

		By("patching the kamaji Deployment with --watch-namespaces=" + watchedNs)
		setManagerArgs(append(append([]string(nil), originalArgs...), "--watch-namespaces="+watchedNs))
	})

	It("reconciles a TenantControlPlane in a watched namespace", func() {
		tcp := tcpFor("tcp-watched", watchedNs)

		createTCPWithRetry(tcp)
		DeferCleanup(func() {
			_ = k8sClient.Delete(context.Background(), tcp)
		})

		Eventually(func() *kamajiv1alpha1.KubernetesVersionStatus {
			return statusOf(tcp)
		}, reconcileWindow, 2*time.Second).ShouldNot(BeNil(), "operator must reconcile TCPs in watched namespaces")
	})

	It("ignores a TenantControlPlane in an unwatched namespace", func() {
		tcp := tcpFor("tcp-ignored", unwatchedNs)

		createTCPWithRetry(tcp)
		DeferCleanup(func() {
			_ = k8sClient.Delete(context.Background(), tcp)
		})

		Consistently(func() *kamajiv1alpha1.KubernetesVersionStatus {
			return statusOf(tcp)
		}, ignoreWindow, 5*time.Second).Should(BeNil(), "operator must not touch TCPs outside the watched namespaces")
	})
})
