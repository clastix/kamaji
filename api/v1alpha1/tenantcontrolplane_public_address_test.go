// Copyright 2025 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TenantControlPlane PublicControlPlaneAddress", func() {
	var tcp *TenantControlPlane

	BeforeEach(func() {
		tcp = &TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "test-namespace",
			},
			Spec: TenantControlPlaneSpec{
				NetworkProfile: NetworkProfileSpec{
					Port: 6443,
				},
				ControlPlane: ControlPlane{
					Service: ServiceSpec{
						ServiceType: ServiceTypeLoadBalancer,
					},
				},
			},
			Status: TenantControlPlaneStatus{
				ControlPlaneEndpoint: "192.168.1.100:6443",
			},
		}
	})

	Context("when PublicAPIServerAddress is not specified", func() {
		It("should fall back to AssignedControlPlaneAddress", func() {
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("192.168.1.100"))
			Expect(port).To(Equal(int32(6443)))
		})

		It("should use default port 6443 when NetworkProfile.Port is not set", func() {
			tcp.Spec.NetworkProfile.Port = 0
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("192.168.1.100"))
			Expect(port).To(Equal(int32(6443)))
		})

		It("should use custom port when NetworkProfile.Port is set", func() {
			tcp.Spec.NetworkProfile.Port = 8443
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("192.168.1.100"))
			Expect(port).To(Equal(int32(8443)))
		})
	})

	Context("when PublicAPIServerAddress is specified", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Service.PublicAPIServerAddress = "k8s-api.example.com"
		})

		It("should return the public address instead of assigned address", func() {
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("k8s-api.example.com"))
			Expect(port).To(Equal(int32(6443)))
		})

		It("should use default port 6443 when NetworkProfile.Port is not set", func() {
			tcp.Spec.NetworkProfile.Port = 0
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("k8s-api.example.com"))
			Expect(port).To(Equal(int32(6443)))
		})

		It("should use custom port when NetworkProfile.Port is set", func() {
			tcp.Spec.NetworkProfile.Port = 8443
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("k8s-api.example.com"))
			Expect(port).To(Equal(int32(8443)))
		})

		It("should still work when ControlPlaneEndpoint is empty", func() {
			tcp.Status.ControlPlaneEndpoint = ""
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("k8s-api.example.com"))
			Expect(port).To(Equal(int32(6443)))
		})
	})

	Context("when PublicAPIServerAddress is empty string", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Service.PublicAPIServerAddress = ""
		})

		It("should fall back to AssignedControlPlaneAddress", func() {
			address, port, err := tcp.PublicControlPlaneAddress()
			Expect(err).NotTo(HaveOccurred())
			Expect(address).To(Equal("192.168.1.100"))
			Expect(port).To(Equal(int32(6443)))
		})
	})

	Context("error handling", func() {
		It("should return error when both PublicAPIServerAddress is empty and ControlPlaneEndpoint is not available", func() {
			tcp.Status.ControlPlaneEndpoint = ""
			_, _, err := tcp.PublicControlPlaneAddress()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("the Tenant Control Plane is not yet exposed"))
		})

		It("should return error when ControlPlaneEndpoint is malformed and PublicAPIServerAddress is empty", func() {
			tcp.Status.ControlPlaneEndpoint = "invalid-endpoint"
			_, _, err := tcp.PublicControlPlaneAddress()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot split host port"))
		})
	})
})