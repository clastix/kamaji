// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with wrong preferred kubelet address type entries", func() {
	It("should fail when using duplicates", func() {
		Consistently(func() error {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicated-kubelet-preferred-address-type",
					Namespace: "default",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					DataStore: "default",
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							Replicas: 1,
						},
						Service: kamajiv1alpha1.ServiceSpec{
							ServiceType: "ClusterIP",
						},
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							PreferredAddressTypes: []kamajiv1alpha1.KubeletPreferredAddressType{
								kamajiv1alpha1.NodeHostName,
								kamajiv1alpha1.NodeInternalIP,
								kamajiv1alpha1.NodeExternalIP,
								kamajiv1alpha1.NodeHostName,
							},
							CGroupFS: "cgroupfs",
						},
					},
				},
			}

			return k8sClient.Create(context.Background(), tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})

	It("should fail when using non valid entries", func() {
		Consistently(func() error {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicated-kubelet-preferred-address-type",
					Namespace: "default",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					DataStore: "default",
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							Replicas: 1,
						},
						Service: kamajiv1alpha1.ServiceSpec{
							ServiceType: "ClusterIP",
						},
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							PreferredAddressTypes: []kamajiv1alpha1.KubeletPreferredAddressType{
								kamajiv1alpha1.NodeHostName,
								kamajiv1alpha1.NodeInternalIP,
								kamajiv1alpha1.NodeExternalIP,
								"Foo",
							},
							CGroupFS: "cgroupfs",
						},
					},
				},
			}

			return k8sClient.Create(context.Background(), tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})
})
