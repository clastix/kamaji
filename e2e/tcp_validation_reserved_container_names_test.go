// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("using a container name reserved by Kamaji", func() {
	newTCP := func(name string) *kamajiv1alpha1.TenantControlPlane {
		return &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
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
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: kamajiv1alpha1.DefaultKubernetesVersion,
				},
			},
		}
	}

	// There is no Deployment yet on creation: the dry-run of the resulting Deployment can't
	// catch the duplicate name, so the reserved names must be rejected upfront.
	It("should block an additional container on creation", func() {
		tcp := newTCP("reserved-additional-container")
		tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{
			Name:  "kube-apiserver",
			Image: "mocked",
		}}
		DeferCleanup(func() {
			Expect(ctrlclient.IgnoreNotFound(k8sClient.Delete(context.Background(), tcp))).To(Succeed())
		})

		Expect(k8sClient.Create(context.Background(), tcp)).To(MatchError(ContainSubstring(`"kube-apiserver" is reserved`)))
	})

	It("should block an additional init container on creation", func() {
		tcp := newTCP("reserved-additional-init-container")
		tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = []corev1.Container{{
			Name:  "kine",
			Image: "mocked",
		}}
		DeferCleanup(func() {
			Expect(ctrlclient.IgnoreNotFound(k8sClient.Delete(context.Background(), tcp))).To(Succeed())
		})

		Expect(k8sClient.Create(context.Background(), tcp)).To(MatchError(ContainSubstring(`"kine" is reserved`)))
	})
})
