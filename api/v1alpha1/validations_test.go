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

var _ = Describe("Datastores validation test", func() {
	var (
		ctx context.Context
		ds  *DataStore
	)

	BeforeEach(func() {
		ctx = context.Background()
		ds = &DataStore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ds",
				Namespace: "default",
			},
			Spec: DataStoreSpec{},
		}
	})

	AfterEach(func() {
		if err := k8sClient.Delete(ctx, ds); err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("DataStores fields", func() {
		It("datastores of type ETCD must have their TLS configurations set correctly", func() {
			ds = &DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-etcd",
				},
				Spec: DataStoreSpec{
					Driver:    "etcd",
					Endpoints: []string{"etcd-server:2379"},
					TLSConfig: &TLSConfig{
						CertificateAuthority: CertKeyPair{},
						ClientCertificate:    &ClientCertificate{},
					},
				},
			}

			err := k8sClient.Create(ctx, ds)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("certificateAuthority privateKey must have secretReference or content when driver is etcd"))
		})

		It("valid ETCD DataStore should be created", func() {
			var (
				cert = []byte("cert")
				key  = []byte("privkey")
			)

			ds = &DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "good-etcd",
				},
				Spec: DataStoreSpec{
					Driver:    "etcd",
					Endpoints: []string{"etcd-server:2379"},
					TLSConfig: &TLSConfig{
						CertificateAuthority: CertKeyPair{
							Certificate: ContentRef{
								Content: cert,
							},
							PrivateKey: &ContentRef{
								Content: key,
							},
						},
						ClientCertificate: &ClientCertificate{
							Certificate: ContentRef{
								Content: cert,
							},
							PrivateKey: ContentRef{
								Content: key,
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, ds)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("datastores of type PostgreSQL must have either basicAuth or tlsConfig", func() {
			ds = &DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-pg",
				},
				Spec: DataStoreSpec{
					Driver:    "PostgreSQL",
					Endpoints: []string{"pg-server:5432"},
				},
			}

			err := k8sClient.Create(ctx, ds)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("When driver is not etcd, either tlsConfig or basicAuth must be provided"))
		})

		It("datastores of type PostgreSQL can have basicAuth", func() {
			ds = &DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "good-pg",
				},
				Spec: DataStoreSpec{
					Driver:    "PostgreSQL",
					Endpoints: []string{"pg-server:5432"},
					BasicAuth: &BasicAuth{
						Username: ContentRef{
							Content: []byte("postgres"),
						},
						Password: ContentRef{
							Content: []byte("postgres"),
						},
					},
				},
			}

			err := k8sClient.Create(ctx, ds)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("datastores of type PostgreSQL must have tlsConfig with proper content", func() {
			ds = &DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-pg",
				},
				Spec: DataStoreSpec{
					Driver:    "PostgreSQL",
					Endpoints: []string{"pg-server:5432"},
					TLSConfig: &TLSConfig{
						ClientCertificate: &ClientCertificate{},
					},
				},
			}

			err := k8sClient.Create(context.Background(), ds)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("When driver is not etcd and tlsConfig exists, clientCertificate must be null or contain valid content"))
		})

		It("datastores of type PostgreSQL need a proper clientCertificate", func() {
			ds = &DataStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "good-pg",
				},
				Spec: DataStoreSpec{
					Driver:    "PostgreSQL",
					Endpoints: []string{"pg-server:5432"},
					TLSConfig: &TLSConfig{
						ClientCertificate: &ClientCertificate{
							Certificate: ContentRef{
								Content: []byte("cert"),
							},
						},
					},
				},
			}

			err := k8sClient.Create(context.Background(), ds)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
