// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/builders/controlplane"
)

var _ = Describe("TCP Deployment webhook", func() {
	var (
		ctx            context.Context
		scheme         *runtime.Scheme
		objects        []client.Object
		failCARead     bool
		createDryRuns  int
		updateDryRuns  int
		tcp, oldTCP    *kamajiv1alpha1.TenantControlPlane
		admissionCheck func() error
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(kamajiv1alpha1.AddToScheme(scheme))

		failCARead, createDryRuns, updateDryRuns = false, 0, 0

		objects = []client.Object{&kamajiv1alpha1.DataStore{ObjectMeta: metav1.ObjectMeta{Name: "default"}}}

		oldTCP = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "default"},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				DataStore:  "default",
				Kubernetes: kamajiv1alpha1.KubernetesSpec{Version: "v1.35.0"},
			},
		}
		oldTCP.Status.Certificates.CA.SecretName = "tcp-ca"

		tcp = oldTCP.DeepCopy()
		tcp.Spec.ControlPlane.Deployment.AdditionalContainers = []corev1.Container{{Name: "sidecar", Image: "busybox"}}

		admissionCheck = func() error {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, isSecret := obj.(*corev1.Secret); isSecret && failCARead {
						return errors.New("connection refused")
					}

					return c.Get(ctx, key, obj, opts...)
				},
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					createOptions := &client.CreateOptions{}
					createOptions.ApplyOptions(opts)
					Expect(createOptions.DryRun).To(ContainElement(metav1.DryRunAll), "the webhook must never persist the Deployment")

					createDryRuns++

					return c.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					updateOptions := &client.UpdateOptions{}
					updateOptions.ApplyOptions(opts)
					Expect(updateOptions.DryRun).To(ContainElement(metav1.DryRunAll), "the webhook must never persist the Deployment")

					updateDryRuns++

					return c.Update(ctx, obj, opts...)
				},
			}).Build()

			handler := TenantControlPlaneDeployment{
				Client:            fakeClient,
				DeploymentBuilder: controlplane.Deployment{Client: fakeClient},
			}

			_, err := handler.OnUpdate(tcp, oldTCP)(ctx, admission.Request{})

			return err
		}
	})

	It("validates with a dry-run create when the Deployment does not exist yet", func() {
		Expect(admissionCheck()).To(Succeed())
		Expect(createDryRuns).To(Equal(1))
		Expect(updateDryRuns).To(BeZero())
	})

	It("validates with a dry-run update when the Deployment exists", func() {
		objects = append(objects, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "default"}})

		Expect(admissionCheck()).To(Succeed())
		Expect(createDryRuns).To(BeZero())
		Expect(updateDryRuns).To(Equal(1))
	})

	It("rejects the update when the CA Secret cannot be read", func() {
		failCARead = true

		Expect(admissionCheck()).To(MatchError(ContainSubstring("connection refused")))
		Expect(createDryRuns + updateDryRuns).To(BeZero())
	})
})
