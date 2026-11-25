// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Controlplane Deployment additional containers", func() {
	var d Deployment

	BeforeEach(func() {
		d = Deployment{}
	})

	// helper: find a container by name in a built PodSpec
	containerByName := func(spec *corev1.PodSpec, name string) corev1.Container {
		for _, c := range spec.Containers {
			if c.Name == name {
				return c
			}
		}
		Fail("container not found: " + name)

		return corev1.Container{}
	}

	Describe("setAdditionalContainers", func() {
		existingPodSpec := func() *corev1.PodSpec {
			return &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "kube-apiserver", Image: "kamaji:latest"},
					{Name: "kube-controller-manager", Image: "kamaji:latest"},
					{Name: "kube-scheduler", Image: "kamaji:latest"},
					{Name: "kine", Image: "rancher/kine:v0.13.9"},
					{Name: "konnectivity-agent", Image: "kamaji:latest"},
				},
			}
		}

		It("drops the Kamaji-built container already provided as additional container", func() {
			podSpec := existingPodSpec()
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{Name: "kine", Image: "ghcr.io/k3s-io/kine:v0.17.6"},
			}

			d.setAdditionalContainers(podSpec, tcp)

			Expect(podSpec.Containers).To(HaveLen(5))
			Expect(containerByName(podSpec, "kine").Image).To(Equal("ghcr.io/k3s-io/kine:v0.17.6"))
			Expect(containerByName(podSpec, "kube-apiserver").Image).To(Equal("kamaji:latest"))
			Expect(containerByName(podSpec, "konnectivity-agent").Image).To(Equal("kamaji:latest"))
		})

		It("keeps every existing container when no additional container collides", func() {
			podSpec := existingPodSpec()
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{Name: "sidecar", Image: "busybox:1.36"},
			}

			d.setAdditionalContainers(podSpec, tcp)

			Expect(podSpec.Containers).To(HaveLen(6))
			Expect(containerByName(podSpec, "kine").Image).To(Equal("rancher/kine:v0.13.9"))
			Expect(containerByName(podSpec, "sidecar").Image).To(Equal("busybox:1.36"))
		})
	})

	Describe("setInitContainers", func() {
		It("drops the Kamaji-built init container already provided as additional init container", func() {
			podSpec := &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "chmod", Image: "rancher/kine:v0.13.9"},
				},
			}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = []corev1.Container{
				{Name: "chmod", Image: "ghcr.io/k3s-io/kine:v0.17.6"},
			}

			d.setInitContainers(podSpec, tcp)

			Expect(podSpec.InitContainers).To(HaveLen(1))
			Expect(podSpec.InitContainers[0].Name).To(Equal("chmod"))
			Expect(podSpec.InitContainers[0].Image).To(Equal("ghcr.io/k3s-io/kine:v0.17.6"))
		})
	})
})
