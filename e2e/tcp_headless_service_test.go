// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with a headless control plane Service", func() {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-headless-service",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
					Headless:    pointer.To(true),
				},
			},
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				// A headless Service has no VIP to announce: the control plane
				// endpoint must come from the declared address.
				Address: "172.18.0.2",
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: kamajiv1alpha1.DefaultKubernetesVersion,
				Kubelet: kamajiv1alpha1.KubeletSpec{
					ConfigurationJSONPatches: []kamajiv1alpha1.JSONPatch{
						{
							Op:   "replace",
							Path: "/cgroupDriver",
							Value: &apiextensionsv1.JSON{
								Raw: []byte(`"cgroupfs"`),
							},
						},
					},
				},
				AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
					"LimitRanger",
					"ResourceQuota",
				},
			},
			Addons: kamajiv1alpha1.AddonsSpec{},
		},
	}

	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	It("Should be Ready with a headless Service and the declared endpoint", func() {
		By("waiting for the TenantControlPlane to be Ready")
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

		By("checking the generated Service is headless")
		svc := &corev1.Service{}
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, svc)).NotTo(HaveOccurred())
		Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))

		By("checking the status endpoint announces the declared address")
		Eventually(func() string {
			if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), tcp); err != nil {
				return ""
			}

			return tcp.Status.ControlPlaneEndpoint
		}).Should(Equal("172.18.0.2:6443"))
	})
})
