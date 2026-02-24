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
	})
})
