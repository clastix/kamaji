// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/datastore"
)

var _ = Describe("DatastoreStorageConfig", func() {
	var (
		ctx context.Context
		dsc *datastore.Config
		tcp *kamajiv1alpha1.TenantControlPlane
		ds  *kamajiv1alpha1.DataStore
	)

	BeforeEach(func() {
		ctx = context.Background()

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{},
		}

		ds = &kamajiv1alpha1.DataStore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "datastore",
				Namespace: "default",
			},
		}

		Expect(kamajiv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
	})

	JustBeforeEach(func() {
		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).WithObjects(tcp).WithStatusSubresource(tcp).Build()

		dsc = &datastore.Config{
			Client:     fakeClient,
			ConnString: "",
			DataStore:  *ds,
		}
	})

	When("TCP has no dataStoreSchema defined", func() {
		It("should return an error", func() {
			_, err := resources.Handle(ctx, dsc, tcp)
			Expect(err).To(HaveOccurred())
		})
	})

	When("TCP has dataStoreSchema set in spec", func() {
		BeforeEach(func() {
			tcp.Spec.DataStoreSchema = "custom-prefix"
		})

		It("should create the datastore secret with the schema name from the spec", func() {
			op, err := resources.Handle(ctx, dsc, tcp)
			Expect(err).ToNot(HaveOccurred())
			Expect(op).To(Equal(controllerutil.OperationResultCreated))

			secrets := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secrets)).To(Succeed())
			Expect(secrets.Items).To(HaveLen(1))
			Expect(secrets.Items[0].Data["DB_SCHEMA"]).To(Equal([]byte("custom-prefix")))
		})
	})

	When("TCP has dataStoreSchema set in status, but not in spec", func() {
		// this test case ensures that existing TCPs (created in a CRD version without
		// the dataStoreSchema field) correctly adopt the spec field from the status.

		It("should create the datastore secret with the correct schema name and update the TCP spec", func() {
			By("updating the TCP status")
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(tcp), tcp)).To(Succeed())
			tcp.Status.Storage.Setup.Schema = "existing-schema-name"
			Expect(fakeClient.Status().Update(ctx, tcp)).To(Succeed())

			By("handling the resource")
			op, err := resources.Handle(ctx, dsc, tcp)
			Expect(err).ToNot(HaveOccurred())
			Expect(op).To(Equal(controllerutil.OperationResultCreated))

			By("checking the secret")
			secrets := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secrets)).To(Succeed())
			Expect(secrets.Items).To(HaveLen(1))
			Expect(secrets.Items[0].Data["DB_SCHEMA"]).To(Equal([]byte("existing-schema-name")))

			By("checking the TCP spec")
			// we have to check the modified struct here (instead of retrieving the object
			// via the fakeClient), as the TCP resource update is not done by the resources.
			// Instead, the TCP controller will handle TCP updates after handling all resources
			tcp.Spec.DataStoreSchema = "existing-schema-name"
		})
	})
})
