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

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with resource with custom service account", func() {
	// service account object
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	// Fill TenantControlPlane object
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-clusterip-customsa",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas:           pointer.To(int32(1)),
					ServiceAccountName: sa.GetName(),
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

	// Create service account and TenantControlPlane resources into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), sa)).NotTo(HaveOccurred())
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	// Delete the service account and TenantControlPlane resources after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})
	// Check if TenantControlPlane resource has been created and if its pods have the right service account
	It("Should be Ready and have correct sa", func() {
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		PodsServiceAccountMustEqualTo(tcp, sa)
	})
})
