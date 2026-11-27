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
					{Name: "konnectivity-server", Image: "registry.k8s.io/kas-network-proxy/proxy-server:v0.0.37"},
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
			Expect(containerByName(podSpec, "konnectivity-server").Image).To(Equal("registry.k8s.io/kas-network-proxy/proxy-server:v0.0.37"))
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

		It("does not replace the Konnectivity server, which a separate resource enriches", func() {
			podSpec := existingPodSpec()
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{Name: "konnectivity-server", Image: "custom-proxy-server:latest"},
			}

			d.setAdditionalContainers(podSpec, tcp)

			// The Konnectivity server is enriched by its own resource in a separate
			// write: replacing it here would persist a bare container, so the managed
			// one is kept and the resulting duplicate is left to the validating
			// webhook to refuse.
			declared := 0
			for _, c := range podSpec.Containers {
				if c.Name == "konnectivity-server" {
					declared++
				}
			}
			Expect(podSpec.Containers).To(HaveLen(6))
			Expect(declared).To(Equal(2))
		})

		It("keeps the managed kube-apiserver when the user declares an additional one", func() {
			podSpec := existingPodSpec()
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{Name: "kube-apiserver", Image: "mocked"},
			}

			d.setAdditionalContainers(podSpec, tcp)

			// Only kine can be replaced: the carried-over kube-apiserver stays, and the
			// resulting duplicate is refused by the validating webhook, as the e2e
			// tcp_additional_resources_blocked_test.go expects.
			declared := 0
			for _, c := range podSpec.Containers {
				if c.Name == "kube-apiserver" {
					declared++
				}
			}
			Expect(podSpec.Containers).To(HaveLen(6))
			Expect(declared).To(Equal(2))
		})

		It("drops user containers declared after a kube-apiserver override once removed from the spec", func() {
			// On creation, a kube-apiserver override is enriched in place by the builder:
			// only the Kamaji-built containers are carried over, whatever the position of the
			// user-space ones, so the sidecar must not survive its removal from the spec.
			podSpec := &corev1.PodSpec{}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{Name: "kube-apiserver", Image: "custom-apiserver:latest"},
				{Name: "sidecar", Image: "busybox:1.36"},
			}

			d.setAdditionalContainers(podSpec, tcp)

			names := make([]string, 0, len(podSpec.Containers))
			for _, c := range podSpec.Containers {
				names = append(names, c.Name)
			}
			Expect(names).To(Equal([]string{"kube-apiserver", "sidecar"}))

			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = nil
			d.setAdditionalContainers(podSpec, tcp)

			names = make([]string, 0, len(podSpec.Containers))
			for _, c := range podSpec.Containers {
				names = append(names, c.Name)
			}
			Expect(names).To(Equal([]string{"kube-apiserver"}))
		})

		It("drops additional containers removed from the spec on the next build", func() {
			podSpec := existingPodSpec()
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{Name: "sidecar", Image: "busybox:1.36"},
			}

			d.setAdditionalContainers(podSpec, tcp)

			// The user removes the additional containers from the TenantControlPlane
			// spec: the next reconcile must not carry them over.
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = nil
			d.setAdditionalContainers(podSpec, tcp)

			names := make([]string, 0, len(podSpec.Containers))
			for _, c := range podSpec.Containers {
				names = append(names, c.Name)
			}
			Expect(names).To(Equal([]string{
				"kube-apiserver", "kube-controller-manager", "kube-scheduler", "kine", "konnectivity-server",
			}))
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

		It("keeps the declared order and drops removed init containers on the next build", func() {
			podSpec := &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "chmod", Image: "rancher/kine:v0.13.9"},
				},
			}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = []corev1.Container{
				{Name: "chmod", Image: "ghcr.io/k3s-io/kine:v0.17.6"},
				{Name: "init-sidecar", Image: "busybox:1.36"},
			}

			d.setInitContainers(podSpec, tcp)

			Expect(podSpec.InitContainers).To(HaveLen(2))
			Expect(podSpec.InitContainers[0].Name).To(Equal("chmod"))
			Expect(podSpec.InitContainers[1].Name).To(Equal("init-sidecar"))

			tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = nil
			d.setInitContainers(podSpec, tcp)

			Expect(podSpec.InitContainers).To(HaveLen(1))
			Expect(podSpec.InitContainers[0].Name).To(Equal("chmod"))
		})
	})
})
