// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/upgrade"
)

var _ = Describe("using an unsupported TenantControlPlane Kubernetes version", func() {
	v, err := semver.Make(upgrade.KubeadmVersion[1:])
	Expect(err).ToNot(HaveOccurred())

	unsupported, err := semver.Make(fmt.Sprintf("%d.%d.%d", v.Major, v.Minor+1, 0))
	Expect(err).ToNot(HaveOccurred())

	It("should be blocked on creation", func() {
		Consistently(func() error {
			tcp := kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unsupported-version",
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
						Version: fmt.Sprintf("v%s", unsupported.String()),
						Kubelet: kamajiv1alpha1.KubeletSpec{
							CGroupFS: "cgroupfs",
						},
					},
				},
			}

			return k8sClient.Create(context.Background(), &tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})

	It("should be blocked on update", func() {
		tcp := kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-linear-update",
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
					Version: fmt.Sprintf("v%s", v.String()),
					Kubelet: kamajiv1alpha1.KubeletSpec{
						CGroupFS: "cgroupfs",
					},
				},
			},
		}

		Expect(k8sClient.Create(context.Background(), &tcp)).ToNot(HaveOccurred())
		defer func() {
			Expect(k8sClient.Delete(context.Background(), &tcp)).ToNot(HaveOccurred())
		}()

		Consistently(func() error {
			tcp := tcp.DeepCopy()

			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, tcp)
			if err != nil {
				return nil
			}

			tcp.Spec.Kubernetes.Version = fmt.Sprintf("v%s", unsupported.String())

			return k8sClient.Create(context.Background(), tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})
})
