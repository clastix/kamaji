// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DataStore endpoints validation", func() {
	var (
		ctx context.Context
		ds  *DataStore
	)

	BeforeEach(func() {
		ctx = context.Background()
		ds = &DataStore{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "ds-endpoints-"},
			Spec: DataStoreSpec{
				Driver: KinePostgreSQLDriver,
				// basicAuth with inline content satisfies the non-etcd auth CEL rules.
				BasicAuth: &BasicAuth{
					Username: ContentRef{Content: []byte("user")},
					Password: ContentRef{Content: []byte("pass")},
				},
			},
		}
	})

	AfterEach(func() {
		if ds.GetName() != "" {
			if err := k8sClient.Delete(ctx, ds); err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	It("accepts an IPv4 host:port endpoint", func() {
		ds.Spec.Endpoints = Endpoints{"10.0.0.1:2379"}
		Expect(k8sClient.Create(ctx, ds)).To(Succeed())
	})

	It("accepts an FQDN host:port endpoint", func() {
		ds.Spec.Endpoints = Endpoints{"postgresql.example.com:5432"}
		Expect(k8sClient.Create(ctx, ds)).To(Succeed())
	})

	It("accepts a bracketed IPv6 host:port endpoint", func() {
		ds.Spec.Endpoints = Endpoints{"[2001:db8::1]:2379"}
		Expect(k8sClient.Create(ctx, ds)).To(Succeed())
	})

	It("denies a bare (unbracketed) IPv6 endpoint", func() {
		ds.Spec.Endpoints = Endpoints{"2001:db8::1:2379"}

		err := k8sClient.Create(ctx, ds)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("each endpoint must be host:port"))
	})
})
