// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane resource", func() {
	// Fill TenantControlPlane object
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-clusterip",
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
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				Address: "172.18.0.2",
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

	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})
	// Check if TenantControlPlane resource has been created
	It("Should be Ready", func() {
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		// ObservedGeneration is set at the end of successful reconciliation,
		// after status becomes Ready, so we need to wait for it.
		Eventually(func() int64 {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, tcp); err != nil {
				return -1
			}

			return tcp.Status.ObservedGeneration
		}, 30*time.Second, time.Second).Should(Equal(tcp.Generation),
			"ObservedGeneration should equal Generation after successful reconciliation")
	})
})
