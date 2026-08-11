// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources/datastore"
)

var _ = Describe("DatastoreMigrate", func() {
	var (
		ctx context.Context
		tcp *kamajiv1alpha1.TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
				UID:       types.UID("test-uid-1234"),
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				DataStore: "kamaji-etcd",
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Storage: kamajiv1alpha1.StorageStatus{
					DataStoreName: "default",
				},
			},
		}

		Expect(kamajiv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())
	})

	It("runs the migration job with a restricted-compliant securityContext", func() {
		actualDS := &kamajiv1alpha1.DataStore{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		}
		desiredDS := &kamajiv1alpha1.DataStore{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kamaji-etcd",
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(tcp, actualDS, desiredDS).
			WithStatusSubresource(tcp).
			Build()

		m := &datastore.Migrate{
			Client:          fakeClient,
			KamajiNamespace: "kamaji",
			MigrateImage:    "kamaji:edge",
		}
		Expect(m.Define(ctx, tcp)).To(Succeed())

		// On first create the result is OperationResultEnqueueBack, not an error.
		_, err := m.CreateOrUpdate(ctx, tcp)
		Expect(err).To(Succeed())

		job := &batchv1.Job{}
		Expect(fakeClient.Get(ctx, types.NamespacedName{Name: "migrate-" + string(tcp.UID), Namespace: "kamaji"}, job)).To(Succeed())

		Expect(job.Spec.Template.Spec.SecurityContext).To(Equal(&corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		}))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsUser).To(BeNil())

		Expect(job.Spec.Template.Spec.Containers[0].SecurityContext).To(Equal(&corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		}))
	})
})
