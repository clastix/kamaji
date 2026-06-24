// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

var _ = Describe("TCP Defaulting Webhook", func() {
	var (
		ctx context.Context
		t   handlers.TenantControlPlaneDefaults
		tcp *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		t = handlers.TenantControlPlaneDefaults{
			DefaultDatastore: "etcd",
		}
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp",
				Namespace: "default",
				UID:       uuid.NewUUID(),
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					ServiceCIDR: "10.96.0.0/12",
					DNSServiceIPs: []string{
						"10.96.0.10",
					},
				},
			},
		}
		ctx = context.Background()
	})

	Describe("fields missing", func() {
		It("should issue all required patches", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(HaveLen(5))
		})

		It("should default the dataStore", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/dataStore", Value: "etcd"},
			))
		})

		It("should default the dataStoreSchema and dataStoreUsername to the expected value", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/dataStoreSchema", Value: string(tcp.UID)},
			))
			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/dataStoreUsername", Value: string(tcp.UID)},
			))
		})
	})

	Describe("fields are already set", func() {
		BeforeEach(func() {
			tcp.Spec.DataStore = "etcd"
			tcp.Spec.DataStoreSchema = "my_tcp"
			tcp.Spec.DataStoreUsername = "my_tcp"
			tcp.Spec.ControlPlane.Deployment.Replicas = ptr.To(int32(2))
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{"10.96.0.0/12"}
		})

		It("should not issue any patches", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeEmpty())
		})
	})

	Describe("backward compatibility", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = nil
			tcp.Spec.NetworkProfile.PodCIDRs = nil
			tcp.Spec.NetworkProfile.PodCIDR = "10.244.0.0/16"
		})

		It("should populate serviceCidrs and podCidrs from legacy fields", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())

			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/networkProfile/serviceCidrs", Value: []interface{}{"10.96.0.0/12"}},
			))

			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/networkProfile/podCidrs", Value: []interface{}{"10.244.0.0/16"}},
			))
		})

		It("should prefer ServiceCIDRs over deprecated ServiceCIDR when both are set", func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{"10.97.0.0/16", "fd00::/120"}

			_, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())

			// extra sanity check: ensure deprecated value is ignored
			Expect(tcp.Spec.NetworkProfile.ServiceCIDRs).To(HaveLen(2))
			Expect(tcp.Spec.NetworkProfile.ServiceCIDRs).To(ContainElements("10.97.0.0/16", "fd00::/120"))
		})
	})

	Describe("dual stack service CIDRs", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDR = ""
			tcp.Spec.NetworkProfile.DNSServiceIPs = nil

			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{
				"10.96.0.0/16",
				"fd00::/120",
			}
		})

		It("should generate DNS IPs for both service CIDRs", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())

			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/networkProfile/dnsServiceIPs", Value: []interface{}{"10.96.0.10", "fd00::10"}},
			))
		})
	})

	Describe("dns service ips already defined", func() {
		BeforeEach(func() {
			tcp.Spec.NetworkProfile.ServiceCIDRs = []string{
				"10.96.0.0/16",
				"fd00::/120",
			}

			tcp.Spec.NetworkProfile.DNSServiceIPs = []string{
				"10.96.0.53",
				"fd00::53",
			}
		})

		It("should preserve existing DNS service IPs", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})

			Expect(err).ToNot(HaveOccurred())

			for _, op := range ops {
				Expect(op.Path).ToNot(Equal("/spec/networkProfile/dnsServiceIPs"))
			}
		})
	})
})
