// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources/datastore"
)

var _ = Describe("DataStore multi-tenancy", func() {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "tenant-a"},
	}

	DescribeTable("CreateOrUpdate",
		func(spec kamajiv1alpha1.DataStoreSpec, usedBy []string, wantErr string) {
			m := &datastore.MultiTenancy{
				DataStore: kamajiv1alpha1.DataStore{
					Spec:   spec,
					Status: kamajiv1alpha1.DataStoreStatus{UsedBy: usedBy},
				},
			}

			_, err := m.CreateOrUpdate(context.Background(), tcp)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("shared etcd used by another tenant",
			kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.EtcdDriver}, []string{"default/tenant-b"}, ""),
		Entry("exclusive etcd not used yet",
			kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.EtcdDriver, Exclusive: true}, nil, ""),
		Entry("exclusive etcd already used by the same tenant",
			kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.EtcdDriver, Exclusive: true}, []string{"default/tenant-a"}, ""),
		Entry("exclusive etcd used by another tenant",
			kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.EtcdDriver, Exclusive: true}, []string{"default/tenant-b"}, "exclusive and already in use"),
		Entry("NATS used by another tenant",
			kamajiv1alpha1.DataStoreSpec{Driver: kamajiv1alpha1.KineNatsDriver}, []string{"default/tenant-b"}, "NATS doesn't support multi-tenancy"),
	)
})
