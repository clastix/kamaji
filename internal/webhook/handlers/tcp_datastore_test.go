// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("TCP Datastore webhook", func() {
	var (
		ctx context.Context
		t   TenantControlPlaneDataStore
		tcp *kamajiv1alpha1.TenantControlPlane
	)
	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(kamajiv1alpha1.AddToScheme(scheme))

		ctx = context.Background()
		t = TenantControlPlaneDataStore{
			Client: fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&kamajiv1alpha1.DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}, &kamajiv1alpha1.DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			}).Build(),
		}
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{},
		}
	})
	Describe("validation should succeed without DataStoreOverrides", func() {
		It("should validate TCP without DataStoreOverrides", func() {
			err := t.checkDataStoreOverrides(ctx, tcp)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("validation should fail with duplicate resources in DataStoreOverrides", func() {
		It("should fail to validate TCP with duplicate resources in DataStoreOverrides", func() {
			tcp.Spec.DataStoreOverrides = []kamajiv1alpha1.DataStoreOverride{{
				Resource:  "/event",
				DataStore: "foo",
			}, {
				Resource:  "/event",
				DataStore: "bar",
			}}
			err := t.checkDataStoreOverrides(ctx, tcp)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("validation should succeed with valid DataStoreOverrides", func() {
		It("should validate TCP with valid DataStoreOverrides", func() {
			tcp.Spec.DataStoreOverrides = []kamajiv1alpha1.DataStoreOverride{{
				Resource:  "/leases",
				DataStore: "foo",
			}, {
				Resource:  "/event",
				DataStore: "bar",
			}}
			err := t.checkDataStoreOverrides(ctx, tcp)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("validation should fail with nonexistent DataStoreOverrides", func() {
		It("should fail to validate TCP with nonexistent DataStoreOverrides", func() {
			tcp.Spec.DataStoreOverrides = []kamajiv1alpha1.DataStoreOverride{{
				Resource:  "/leases",
				DataStore: "baz",
			}}
			err := t.checkDataStoreOverrides(ctx, tcp)
			Expect(err).To(HaveOccurred())
		})
	})
})
