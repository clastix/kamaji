// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	builder "github.com/clastix/kamaji/internal/builders/controlplane"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

var _ = Describe("KubernetesDeploymentResource with Konnectivity", func() {
	It("creates the Deployment with the Konnectivity server in its first revision", func() {
		ctx := context.Background()
		fakeClient := fake.NewClientBuilder().WithScheme(runtimeScheme).Build()
		tcp := &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "test-tcp", Namespace: "default"},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{Replicas: ptr.To(int32(2))},
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{Version: "v1.35.0"},
				Addons: kamajiv1alpha1.AddonsSpec{
					Konnectivity: &kamajiv1alpha1.KonnectivitySpec{
						KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
							Port:  8132,
							Image: "registry.k8s.io/kas-network-proxy/proxy-server",
						},
					},
				},
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Addons: kamajiv1alpha1.AddonsStatus{
					Konnectivity: kamajiv1alpha1.KonnectivityStatus{
						ConfigMap:  kamajiv1alpha1.KonnectivityConfigMap{Name: "test-tcp-konnectivity-egress-selector-configuration"},
						Kubeconfig: kamajiv1alpha1.KubeconfigStatus{SecretName: "test-tcp-konnectivity-kubeconfig"},
					},
				},
			},
		}

		deployment := &resources.KubernetesDeploymentResource{Client: fakeClient}
		Expect(deployment.Define(ctx, tcp)).To(Succeed())
		Expect(deployment.CreateOrUpdate(ctx, tcp)).To(Equal(controllerutil.OperationResultCreated))

		konnectivityPatch := &konnectivity.KubernetesDeploymentResource{
			Builder: builder.Konnectivity{Scheme: *runtimeScheme},
			Client:  fakeClient,
		}
		Expect(konnectivityPatch.Define(ctx, tcp)).To(Succeed())
		Expect(konnectivityPatch.CreateOrUpdate(ctx, tcp)).To(Equal(controllerutil.OperationResultNone))
	})
})
