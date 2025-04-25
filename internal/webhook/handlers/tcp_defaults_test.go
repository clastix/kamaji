// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Expect(ops).To(HaveLen(3))
		})

		It("should default the dataStore", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/dataStore", Value: "etcd"},
			))
		})

		It("should default the dataStoreSchema to the expected value", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/dataStoreSchema", Value: "default_tcp"},
			))
		})
	})

	Describe("fields are already set", func() {
		BeforeEach(func() {
			tcp.Spec.DataStore = "etcd"
			tcp.Spec.DataStoreSchema = "my_tcp"
			tcp.Spec.ControlPlane.Deployment.Replicas = ptr.To(int32(2))
		})

		It("should not issue any patches", func() {
			ops, err := t.OnCreate(tcp)(ctx, admission.Request{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ops).To(BeEmpty())
		})
	})
})
