// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestControlplaneDeployment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controlplane Deployment Suite")
}

var _ = Describe("Controlplane Deployment", func() {
	var d Deployment
	BeforeEach(func() {
		d = Deployment{
			DataStoreOverrides: []DataStoreOverrides{{
				Resource: "/events",
				DataStore: kamajiv1alpha1.DataStore{
					Spec: kamajiv1alpha1.DataStoreSpec{
						Endpoints: kamajiv1alpha1.Endpoints{"etcd-0", "etcd-1", "etcd-2"},
					},
				},
			}},
		}
	})

	Describe("DataStoreOverrides flag generation", func() {
		It("should generate valid --etcd-servers-overrides value", func() {
			etcdSerVersOverrides := d.etcdServersOverrides()
			Expect(etcdSerVersOverrides).To(Equal("/events#https://etcd-0;https://etcd-1;https://etcd-2"))
		})
		It("should generate valid --etcd-servers-overrides value with 2 DataStoreOverrides", func() {
			d.DataStoreOverrides = append(d.DataStoreOverrides, DataStoreOverrides{
				Resource: "/pods",
				DataStore: kamajiv1alpha1.DataStore{
					Spec: kamajiv1alpha1.DataStoreSpec{
						Endpoints: kamajiv1alpha1.Endpoints{"etcd-3", "etcd-4", "etcd-5"},
					},
				},
			})
			etcdSerVersOverrides := d.etcdServersOverrides()
			Expect(etcdSerVersOverrides).To(Equal("/events#https://etcd-0;https://etcd-1;https://etcd-2,/pods#https://etcd-3;https://etcd-4;https://etcd-5"))
		})
	})

	Describe("applyProbeOverrides", func() {
		var probe *corev1.Probe

		BeforeEach(func() {
			probe = &corev1.Probe{
				InitialDelaySeconds: 0,
				TimeoutSeconds:      1,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    3,
			}
		})

		It("should not modify probe when spec is nil", func() {
			applyProbeOverrides(probe, nil)
			Expect(probe.InitialDelaySeconds).To(Equal(int32(0)))
			Expect(probe.TimeoutSeconds).To(Equal(int32(1)))
			Expect(probe.PeriodSeconds).To(Equal(int32(10)))
			Expect(probe.SuccessThreshold).To(Equal(int32(1)))
			Expect(probe.FailureThreshold).To(Equal(int32(3)))
		})

		It("should override only FailureThreshold when only it is set", func() {
			spec := &kamajiv1alpha1.ProbeSpec{
				FailureThreshold: pointer.To(int32(30)),
			}
			applyProbeOverrides(probe, spec)
			Expect(probe.FailureThreshold).To(Equal(int32(30)))
			Expect(probe.InitialDelaySeconds).To(Equal(int32(0)))
			Expect(probe.TimeoutSeconds).To(Equal(int32(1)))
			Expect(probe.PeriodSeconds).To(Equal(int32(10)))
			Expect(probe.SuccessThreshold).To(Equal(int32(1)))
		})

		It("should override all fields when all are set", func() {
			spec := &kamajiv1alpha1.ProbeSpec{
				InitialDelaySeconds: pointer.To(int32(15)),
				TimeoutSeconds:      pointer.To(int32(5)),
				PeriodSeconds:       pointer.To(int32(30)),
				SuccessThreshold:    pointer.To(int32(2)),
				FailureThreshold:    pointer.To(int32(10)),
			}
			applyProbeOverrides(probe, spec)
			Expect(probe.InitialDelaySeconds).To(Equal(int32(15)))
			Expect(probe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(probe.PeriodSeconds).To(Equal(int32(30)))
			Expect(probe.SuccessThreshold).To(Equal(int32(2)))
			Expect(probe.FailureThreshold).To(Equal(int32(10)))
		})

		It("should cascade global then component overrides", func() {
			global := &kamajiv1alpha1.ProbeSpec{
				FailureThreshold: pointer.To(int32(10)),
				PeriodSeconds:    pointer.To(int32(20)),
			}
			applyProbeOverrides(probe, global)

			component := &kamajiv1alpha1.ProbeSpec{
				FailureThreshold: pointer.To(int32(60)),
			}
			applyProbeOverrides(probe, component)

			Expect(probe.FailureThreshold).To(Equal(int32(60)))
			Expect(probe.PeriodSeconds).To(Equal(int32(20)))
			Expect(probe.TimeoutSeconds).To(Equal(int32(1)))
			Expect(probe.InitialDelaySeconds).To(Equal(int32(0)))
			Expect(probe.SuccessThreshold).To(Equal(int32(1)))
		})

		It("should not panic when probe is nil", func() {
			spec := &kamajiv1alpha1.ProbeSpec{PeriodSeconds: pointer.To(int32(20))}
			Expect(func() { applyProbeOverrides(nil, spec) }).ToNot(Panic())
		})
	})

	Describe("mergeAPIServerArgs", func() {
		var (
			safeDefaults map[string]string
			managed      map[string]string
		)

		BeforeEach(func() {
			safeDefaults = map[string]string{
				"--authorization-mode":     "Node,RBAC",
				"--service-account-issuer": "https://kubernetes.default.svc.cluster.local",
			}
			managed = map[string]string{
				"--secure-port":              "6443",
				"--service-cluster-ip-range": "10.96.0.0/12",
			}
		})

		It("applies safe defaults when the user did not provide them", func() {
			got := mergeAPIServerArgs(nil, nil, safeDefaults, managed)
			Expect(got).To(ContainElement("--authorization-mode=Node,RBAC"))
			Expect(got).To(ContainElement("--service-account-issuer=https://kubernetes.default.svc.cluster.local"))
		})

		It("lets the user override a safe default", func() {
			user := []string{"--authorization-mode=AlwaysAllow"}
			got := mergeAPIServerArgs(nil, user, safeDefaults, managed)
			Expect(got).To(ContainElement("--authorization-mode=AlwaysAllow"))
			Expect(got).NotTo(ContainElement("--authorization-mode=Node,RBAC"))
		})

		It("ignores user attempts to override a managed flag", func() {
			user := []string{"--secure-port=9443"}
			got := mergeAPIServerArgs(nil, user, safeDefaults, managed)
			Expect(got).To(ContainElement("--secure-port=6443"))
			Expect(got).NotTo(ContainElement("--secure-port=9443"))
		})

		It("preserves multiple user values for a repeatable flag", func() {
			user := []string{
				"--service-account-issuer=https://issuer-one.example.com",
				"--service-account-issuer=https://issuer-two.example.com",
			}
			got := mergeAPIServerArgs(nil, user, safeDefaults, managed)
			Expect(got).To(ContainElement("--service-account-issuer=https://issuer-one.example.com"))
			Expect(got).To(ContainElement("--service-account-issuer=https://issuer-two.example.com"))
			Expect(got).NotTo(ContainElement("--service-account-issuer=https://kubernetes.default.svc.cluster.local"))
		})

		It("sorts Kamaji-owned flags and appends user extras verbatim at the end", func() {
			user := []string{
				"--service-account-issuer=https://issuer-one.example.com",
				"--service-account-issuer=https://issuer-two.example.com",
				"--audit-log-path=/var/log/audit.log",
			}
			got := mergeAPIServerArgs(nil, user, safeDefaults, managed)
			Expect(got).To(Equal([]string{
				"--authorization-mode=Node,RBAC",
				"--secure-port=6443",
				"--service-cluster-ip-range=10.96.0.0/12",
				"--service-account-issuer=https://issuer-one.example.com",
				"--service-account-issuer=https://issuer-two.example.com",
				"--audit-log-path=/var/log/audit.log",
			}))
		})

		It("preserves foreign flags from current, sorted within the Kamaji-owned segment", func() {
			current := []string{"--egress-selector-config-file=/etc/kubernetes/konnectivity/egress.yaml"}
			got := mergeAPIServerArgs(current, nil, safeDefaults, managed)
			Expect(got).To(Equal([]string{
				"--authorization-mode=Node,RBAC",
				"--egress-selector-config-file=/etc/kubernetes/konnectivity/egress.yaml",
				"--secure-port=6443",
				"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
				"--service-cluster-ip-range=10.96.0.0/12",
			}))
		})
	})

	Describe("control plane probes", func() {
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

		It("renders a readiness probe for kube-scheduler on /healthz:10259", func() {
			podSpec := &corev1.PodSpec{}
			d.buildScheduler(podSpec, kamajiv1alpha1.TenantControlPlane{})

			c := containerByName(podSpec, "kube-scheduler")
			Expect(c.ReadinessProbe).ToNot(BeNil())
			Expect(c.ReadinessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(c.ReadinessProbe.HTTPGet.Port.IntValue()).To(Equal(10259))
			Expect(c.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(c.ReadinessProbe.PeriodSeconds).To(Equal(int32(10)))
		})

		It("renders a readiness probe for kube-controller-manager on /healthz:10257", func() {
			podSpec := &corev1.PodSpec{}
			d.buildControllerManager(podSpec, kamajiv1alpha1.TenantControlPlane{})

			c := containerByName(podSpec, "kube-controller-manager")
			Expect(c.ReadinessProbe).ToNot(BeNil())
			Expect(c.ReadinessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(c.ReadinessProbe.HTTPGet.Port.IntValue()).To(Equal(10257))
			Expect(c.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
		})

		It("cascades global then component readiness overrides onto the scheduler", func() {
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.Probes = &kamajiv1alpha1.ControlPlaneProbes{
				Readiness: &kamajiv1alpha1.ProbeSpec{PeriodSeconds: pointer.To(int32(20))},
				Scheduler: &kamajiv1alpha1.ProbeSet{
					Readiness: &kamajiv1alpha1.ProbeSpec{PeriodSeconds: pointer.To(int32(30))},
				},
			}

			podSpec := &corev1.PodSpec{}
			d.buildScheduler(podSpec, tcp)

			c := containerByName(podSpec, "kube-scheduler")
			Expect(c.ReadinessProbe.PeriodSeconds).To(Equal(int32(30))) // component wins over global
		})

		It("applies a global-only readiness override to the scheduler", func() {
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.Probes = &kamajiv1alpha1.ControlPlaneProbes{
				Readiness: &kamajiv1alpha1.ProbeSpec{PeriodSeconds: pointer.To(int32(20))},
			}

			podSpec := &corev1.PodSpec{}
			d.buildScheduler(podSpec, tcp)

			c := containerByName(podSpec, "kube-scheduler")
			Expect(c.ReadinessProbe.PeriodSeconds).To(Equal(int32(20)))
		})

		It("leaves the kube-apiserver probes unchanged (regression guard)", func() {
			podSpec := &corev1.PodSpec{}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.NetworkProfile.Port = 6443
			d.buildKubeAPIServer(podSpec, tcp, "")

			c := containerByName(podSpec, "kube-apiserver")
			Expect(c.LivenessProbe.HTTPGet.Path).To(Equal("/livez"))
			Expect(c.ReadinessProbe.HTTPGet.Path).To(Equal("/readyz"))
			Expect(c.StartupProbe.HTTPGet.Path).To(Equal("/livez"))
			Expect(c.ReadinessProbe.HTTPGet.Port.IntValue()).To(Equal(6443))
		})
	})
})
