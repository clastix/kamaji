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
	"github.com/clastix/kamaji/internal/utilities"
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

		It("lets the user select a dedicated client CA bundle", func() {
			const clientCAPath = "/etc/kubernetes/client-ca/client-ca-bundle.crt"

			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.ExtraArgs = &kamajiv1alpha1.ControlPlaneExtraArgs{
				APIServer: []string{"--client-ca-file=" + clientCAPath},
			}

			got := d.buildKubeAPIServerCommand(tcp, "10.0.0.1", nil)

			Expect(got).To(ContainElement("--client-ca-file=" + clientCAPath))
			Expect(got).NotTo(ContainElement("--client-ca-file=/etc/kubernetes/pki/ca.crt"))
		})

		It("lets the user select a dedicated client certificate signer", func() {
			const (
				signerCertPath = "/etc/kubernetes/client-ca/client-ca.crt"
				signerKeyPath  = "/etc/kubernetes/client-ca/client-ca.key"
			)

			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.ExtraArgs = &kamajiv1alpha1.ControlPlaneExtraArgs{
				ControllerManager: []string{
					"--cluster-signing-cert-file=" + signerCertPath,
					"--cluster-signing-key-file=" + signerKeyPath,
				},
			}
			podSpec := &corev1.PodSpec{}

			d.buildControllerManager(podSpec, tcp)

			Expect(podSpec.Containers).To(HaveLen(1))
			Expect(podSpec.Containers[0].Args).To(ContainElements(
				"--cluster-signing-cert-file="+signerCertPath,
				"--cluster-signing-key-file="+signerKeyPath,
			))
			Expect(podSpec.Containers[0].Args).NotTo(ContainElements(
				"--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt",
				"--cluster-signing-key-file=/etc/kubernetes/pki/ca.key",
			))
		})

		It("omits global signer flags when per-purpose signers are configured", func() {
			perPurposeArgs := []string{
				"--cluster-signing-kube-apiserver-client-cert-file=/etc/kubernetes/client-ca/ca.crt",
				"--cluster-signing-kube-apiserver-client-key-file=/etc/kubernetes/client-ca/ca.key",
				"--cluster-signing-kubelet-client-cert-file=/etc/kubernetes/client-ca/ca.crt",
				"--cluster-signing-kubelet-client-key-file=/etc/kubernetes/client-ca/ca.key",
				"--cluster-signing-kubelet-serving-cert-file=/etc/kubernetes/pki/ca.crt",
				"--cluster-signing-kubelet-serving-key-file=/etc/kubernetes/pki/ca.key",
				"--cluster-signing-legacy-unknown-cert-file=/etc/kubernetes/client-ca/ca.crt",
				"--cluster-signing-legacy-unknown-key-file=/etc/kubernetes/client-ca/ca.key",
			}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.ExtraArgs = &kamajiv1alpha1.ControlPlaneExtraArgs{
				ControllerManager: perPurposeArgs,
			}
			podSpec := &corev1.PodSpec{}

			d.buildControllerManager(podSpec, tcp)

			Expect(podSpec.Containers).To(HaveLen(1))
			Expect(podSpec.Containers[0].Args).To(ContainElements(perPurposeArgs))
			Expect(podSpec.Containers[0].Args).NotTo(ContainElements(
				"--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt",
				"--cluster-signing-key-file=/etc/kubernetes/pki/ca.key",
			))
		})

		It("mounts client trust and signer secrets into only the components that need them", func() {
			clientTrustVolume := corev1.Volume{
				Name: "client-ca-bundle",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "tenant-client-ca-bundle",
					},
				},
			}
			clientSignerVolume := corev1.Volume{
				Name: "client-ca-signer",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "tenant-client-ca-signer",
					},
				},
			}
			clientTrustMount := corev1.VolumeMount{
				Name:      clientTrustVolume.Name,
				MountPath: "/etc/kubernetes/client-ca",
				ReadOnly:  true,
			}
			clientSignerMount := corev1.VolumeMount{
				Name:      clientSignerVolume.Name,
				MountPath: "/etc/kubernetes/client-signer",
				ReadOnly:  true,
			}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AdditionalVolumes = []corev1.Volume{clientTrustVolume, clientSignerVolume}
			tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts = &kamajiv1alpha1.AdditionalVolumeMounts{
				APIServer:         []corev1.VolumeMount{clientTrustMount},
				ControllerManager: []corev1.VolumeMount{clientSignerMount},
			}
			podSpec := &corev1.PodSpec{}

			d.setAdditionalVolumes(podSpec, tcp)
			d.buildKubeAPIServer(podSpec, tcp, "10.0.0.1")
			d.buildControllerManager(podSpec, tcp)

			Expect(podSpec.Volumes).To(ContainElements(clientTrustVolume, clientSignerVolume))
			Expect(podSpec.Containers[0].VolumeMounts).To(ContainElement(clientTrustMount))
			Expect(podSpec.Containers[0].VolumeMounts).NotTo(ContainElement(clientSignerMount))
			Expect(podSpec.Containers[1].VolumeMounts).To(ContainElement(clientSignerMount))
			Expect(podSpec.Containers[1].VolumeMounts).NotTo(ContainElement(clientTrustMount))
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

	Describe("ServiceAccount", func() {
		It("should default to 'default' SA with nil automount", func() {
			podSpec := &corev1.PodSpec{}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			d.setServiceAccount(podSpec, tcp)
			Expect(podSpec.ServiceAccountName).To(Equal("default"))
			Expect(podSpec.AutomountServiceAccountToken).To(BeNil())
		})

		It("should set a custom SA name with nil automount", func() {
			podSpec := &corev1.PodSpec{}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.ServiceAccountName = "custom-sa"
			d.setServiceAccount(podSpec, tcp)
			Expect(podSpec.ServiceAccountName).To(Equal("custom-sa"))
			Expect(podSpec.AutomountServiceAccountToken).To(BeNil())
		})

		It("should enable automount when AutomountServiceAccountToken is true", func() {
			podSpec := &corev1.PodSpec{}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AutomountServiceAccountToken = pointer.To(true)
			d.setServiceAccount(podSpec, tcp)
			Expect(podSpec.ServiceAccountName).To(Equal("default"))
			Expect(podSpec.AutomountServiceAccountToken).ToNot(BeNil())
			Expect(*podSpec.AutomountServiceAccountToken).To(BeTrue())
		})

		It("should disable automount when AutomountServiceAccountToken is false", func() {
			podSpec := &corev1.PodSpec{}
			tcp := kamajiv1alpha1.TenantControlPlane{}
			tcp.Spec.ControlPlane.Deployment.AutomountServiceAccountToken = pointer.To(false)
			tcp.Spec.ControlPlane.Deployment.ServiceAccountName = "another-sa"
			d.setServiceAccount(podSpec, tcp)
			Expect(podSpec.ServiceAccountName).To(Equal("another-sa"))
			Expect(podSpec.AutomountServiceAccountToken).ToNot(BeNil())
			Expect(*podSpec.AutomountServiceAccountToken).To(BeFalse())
		})
	})

	Describe("Kine container image override", func() {
		var tcp kamajiv1alpha1.TenantControlPlane
		BeforeEach(func() {
			d.KineContainerImage = "rancher/kine:v0.11.0"
			d.DataStore = kamajiv1alpha1.DataStore{
				Spec: kamajiv1alpha1.DataStoreSpec{
					Driver: kamajiv1alpha1.KinePostgreSQLDriver,
				},
			}
			tcp = kamajiv1alpha1.TenantControlPlane{}
			tcp.Status.Storage = kamajiv1alpha1.StorageStatus{
				Config: kamajiv1alpha1.DataStoreConfigStatus{
					SecretName: "test-secret",
				},
			}
		})

		It("should use default kine image when not overridden", func() {
			podSpec := &corev1.PodSpec{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{}

			d.setAdditionalContainers(podSpec, tcp)
			d.buildKine(podSpec, tcp)

			found, index := utilities.HasNamedContainer(podSpec.Containers, "kine")
			Expect(found).To(BeTrue())
			Expect(podSpec.Containers[index].Image).To(Equal("rancher/kine:v0.11.0"))
		})

		It("should use custom kine image when overridden via additionalContainers", func() {
			podSpec := &corev1.PodSpec{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{
					Name:  "kine",
					Image: "my-custom-kine:v1.0.0",
				},
			}

			d.setAdditionalContainers(podSpec, tcp)
			d.buildKine(podSpec, tcp)

			found, index := utilities.HasNamedContainer(podSpec.Containers, "kine")
			Expect(found).To(BeTrue())
			Expect(podSpec.Containers[index].Image).To(Equal("my-custom-kine:v1.0.0"))
		})

		It("should preserve custom kine container image when set via additionalContainers", func() {
			podSpec := &corev1.PodSpec{}
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{
				{
					Name:  "kine",
					Image: "custom-kine:latest",
				},
			}

			d.setAdditionalContainers(podSpec, tcp)
			d.buildKine(podSpec, tcp)

			found, index := utilities.HasNamedContainer(podSpec.Containers, "kine")
			Expect(found).To(BeTrue())
			Expect(podSpec.Containers[index].Image).To(Equal("custom-kine:latest"))
		})
	})
})
