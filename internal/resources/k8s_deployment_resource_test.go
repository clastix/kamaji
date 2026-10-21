// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

var _ = Describe("KubernetesDeploymentResource Konnectivity", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		tcp        *kamajiv1alpha1.TenantControlPlane
	)

	konnectivitySpec := func() *kamajiv1alpha1.KonnectivitySpec {
		return &kamajiv1alpha1.KonnectivitySpec{
			KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
				Port:  8132,
				Image: "registry.k8s.io/kas-network-proxy/proxy-server",
			},
		}
	}

	reconcile := func() controllerutil.OperationResult {
		resource := &resources.KubernetesDeploymentResource{Client: fakeClient}
		Expect(resource.Define(ctx, tcp)).To(Succeed())

		result, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		if result != controllerutil.OperationResultNone || resource.ShouldStatusBeUpdated(ctx, tcp) {
			Expect(resource.UpdateTenantControlPlaneStatus(ctx, tcp)).To(Succeed())
		}

		return result
	}

	podSpec := func() corev1.PodSpec {
		var deployment appsv1.Deployment
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(tcp), &deployment)).To(Succeed())

		return deployment.Spec.Template.Spec
	}

	containerNames := func(spec corev1.PodSpec) []string {
		var names []string
		for _, c := range spec.Containers {
			names = append(names, c.Name)
		}

		return names
	}

	volumeNames := func(spec corev1.PodSpec) []string {
		var names []string
		for _, v := range spec.Volumes {
			names = append(names, v.Name)
		}

		return names
	}

	apiServerArgs := func(spec corev1.PodSpec) string {
		for _, c := range spec.Containers {
			if c.Name == "kube-apiserver" {
				return strings.Join(c.Args, " ")
			}
		}

		return ""
	}

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fake.NewClientBuilder().WithScheme(runtimeScheme).Build()
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "test-tcp", Namespace: "default"},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{Replicas: ptr.To(int32(2))},
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{Version: "v1.35.0"},
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
	})

	It("creates the Deployment with the Konnectivity server in its first revision", func() {
		tcp.Spec.Addons.Konnectivity = konnectivitySpec()

		Expect(reconcile()).To(Equal(controllerutil.OperationResultCreated))
		Expect(containerNames(podSpec())).To(ContainElement("konnectivity-server"))
		Expect(volumeNames(podSpec())).To(ContainElements("konnectivity-uds", "egress-selector-configuration", "konnectivity-server-kubeconfig"))
		Expect(apiServerArgs(podSpec())).To(ContainSubstring("--egress-selector-config-file="))
		Expect(tcp.Status.Addons.Konnectivity.Enabled).To(BeTrue())

		Expect(reconcile()).To(Equal(controllerutil.OperationResultNone))
	})

	It("restores the Deployment in one update when the addon is disabled", func() {
		Expect(reconcile()).To(Equal(controllerutil.OperationResultCreated))
		withoutKonnectivity := podSpec()

		tcp.Spec.Addons.Konnectivity = konnectivitySpec()
		Expect(reconcile()).To(Equal(controllerutil.OperationResultUpdated))
		Expect(containerNames(podSpec())).To(ContainElement("konnectivity-server"))

		tcp.Spec.Addons.Konnectivity = nil

		Expect(reconcile()).To(Equal(controllerutil.OperationResultUpdated))
		Expect(podSpec()).To(Equal(withoutKonnectivity))
		Expect(tcp.Status.Addons.Konnectivity.Enabled).To(BeFalse())

		Expect(reconcile()).To(Equal(controllerutil.OperationResultNone))
	})

	It("leaves a Deployment without Konnectivity unchanged", func() {
		Expect(reconcile()).To(Equal(controllerutil.OperationResultCreated))
		Expect(containerNames(podSpec())).NotTo(ContainElement("konnectivity-server"))
		Expect(tcp.Status.Addons.Konnectivity.Enabled).To(BeFalse())

		Expect(reconcile()).To(Equal(controllerutil.OperationResultNone))
	})

	It("keeps a user egress selector flag when the addon was never enabled", func() {
		tcp.Spec.ControlPlane.Deployment.ExtraArgs = &kamajiv1alpha1.ControlPlaneExtraArgs{
			APIServer: []string{"--egress-selector-config-file=/custom/egress.yaml"},
		}

		Expect(reconcile()).To(Equal(controllerutil.OperationResultCreated))
		Expect(apiServerArgs(podSpec())).To(ContainSubstring("--egress-selector-config-file=/custom/egress.yaml"))

		reconcile()
		Expect(apiServerArgs(podSpec())).To(ContainSubstring("--egress-selector-config-file=/custom/egress.yaml"))
	})
})
