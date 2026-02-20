// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	Describe("startupProbeFailureThreshold", func() {
		It("should return default value when StartupProbeFailureThreshold is nil", func() {
			tcp := kamajiv1alpha1.TenantControlPlane{}
			Expect(startupProbeFailureThreshold(tcp)).To(Equal(int32(3)))
		})

		It("should return the configured value when StartupProbeFailureThreshold is set", func() {
			tcp := kamajiv1alpha1.TenantControlPlane{
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							StartupProbeFailureThreshold: pointer.To(int32(30)),
						},
					},
				},
			}
			Expect(startupProbeFailureThreshold(tcp)).To(Equal(int32(30)))
		})
	})
})
