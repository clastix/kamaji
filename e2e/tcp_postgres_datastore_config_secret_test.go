// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("When the datastore-config Secret is corrupted for a PostgreSQL-backed TenantControlPlane", func() {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgresql-secret-regeneration",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			DataStore: "postgresql-bronze",
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
				},
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.23.6",
			},
		},
	}

	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	It("Should regenerate the Secret and restart the TCP pods successfully", func() {
		By("recording the UIDs of the currently running TenantControlPlane pods")
		initialPodUIDs := sets.New[types.UID]()
		Eventually(func() int {
			podList := &corev1.PodList{}
			if err := k8sClient.List(context.Background(), podList,
				client.InNamespace(tcp.GetNamespace()),
				client.MatchingLabels{"kamaji.clastix.io/name": tcp.GetName()},
			); err != nil {
				return 0
			}

			initialPodUIDs.Clear()
			for _, pod := range podList.Items {
				initialPodUIDs.Insert(pod.GetUID())
			}

			return initialPodUIDs.Len()
		}, time.Minute, time.Second).Should(Not(BeZero()))

		By("retrieving the current datastore-config Secret and its checksum")
		secretName := fmt.Sprintf("%s-datastore-config", tcp.GetName())

		var secret corev1.Secret
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: secretName, Namespace: tcp.GetNamespace()}, &secret)).To(Succeed())

		originalChecksum := secret.GetAnnotations()["kamaji.clastix.io/checksum"]
		Expect(originalChecksum).NotTo(BeEmpty(), "expected datastore-config Secret to carry a checksum annotation")

		By("corrupting the DB_PASSWORD in the datastore-config Secret")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&secret), &secret); err != nil {
				return err
			}

			secret.Data["DB_PASSWORD"] = []byte("corrupted-password")

			return k8sClient.Update(context.Background(), &secret)
		})
		Expect(err).ToNot(HaveOccurred())

		By("waiting for the controller to detect the corruption and regenerate the Secret with a new checksum")
		Eventually(func() string {
			if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&secret), &secret); err != nil {
				return ""
			}

			return secret.GetAnnotations()["kamaji.clastix.io/checksum"]
		}, 5*time.Minute, time.Second).ShouldNot(Equal(originalChecksum))

		By("waiting for at least one new TenantControlPlane pod to replace the pre-existing ones")
		Eventually(func() bool {
			var podList corev1.PodList
			if err := k8sClient.List(context.Background(), &podList,
				client.InNamespace(tcp.GetNamespace()),
				client.MatchingLabels{"kamaji.clastix.io/name": tcp.GetName()},
			); err != nil {
				return false
			}
			for _, pod := range podList.Items {
				if !initialPodUIDs.Has(pod.GetUID()) {
					return true
				}
			}

			return false
		}, 5*time.Minute, time.Second).Should(BeTrue())

		By("verifying the TenantControlPlane is Ready after the restart with the regenerated Secret")
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
	})
})
