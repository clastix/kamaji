// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

var _ = Describe("Konnectivity ServiceResource", func() {
	const (
		tcpName            = "test-tcp"
		konnectivityPort   = int32(8132)
		additionalPort     = int32(50001)
		additionalNodePort = int32(32542)
	)

	var (
		ctx        context.Context
		tcp        *kamajiv1alpha1.TenantControlPlane
		fakeClient client.Client
	)

	newClient := func(objs ...client.Object) client.Client {
		return fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(objs...).Build()
	}

	// reconcile runs the two Service builders in the order the TenantControlPlane controller does.
	reconcile := func() (controllerutil.OperationResult, controllerutil.OperationResult) {
		svc := &resources.KubernetesServiceResource{Client: fakeClient}
		Expect(svc.Define(ctx, tcp)).To(Succeed())
		svcResult, err := svc.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		konn := &konnectivity.ServiceResource{Client: fakeClient}
		Expect(konn.Define(ctx, tcp)).To(Succeed())
		konnResult, err := konn.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		return svcResult, konnResult
	}

	getService := func() *corev1.Service {
		svc := &corev1.Service{}
		Expect(fakeClient.Get(ctx, client.ObjectKey{Name: tcpName, Namespace: "default"}, svc)).To(Succeed())

		return svc
	}

	portNames := func(svc *corev1.Service) []string {
		names := make([]string, 0, len(svc.Spec.Ports))
		for _, port := range svc.Spec.Ports {
			names = append(names, port.Name)
		}

		return names
	}

	BeforeEach(func() {
		ctx = context.Background()
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName, Namespace: "default"},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer,
						AdditionalPorts: []kamajiv1alpha1.AdditionalPort{{
							Name:       "talos-trustd",
							Protocol:   corev1.ProtocolTCP,
							Port:       additionalPort,
							TargetPort: intstr.FromInt32(additionalPort),
						}},
					},
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{Port: 6443},
				Addons: kamajiv1alpha1.AddonsSpec{
					Konnectivity: &kamajiv1alpha1.KonnectivitySpec{
						KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{Port: konnectivityPort},
					},
				},
			},
		}
	})

	It("keeps the additional ports on the reconcile that creates the Service", func() {
		fakeClient = newClient()

		reconcile()

		Expect(portNames(getService())).To(Equal([]string{"kube-apiserver", "konnectivity-server", "talos-trustd"}))

		svcResult, konnResult := reconcile()
		Expect(svcResult).To(Equal(controllerutil.OperationResultNone))
		Expect(konnResult).To(Equal(controllerutil.OperationResultNone))
	})

	It("keeps an additional port and its fields when Konnectivity is enabled on an existing Service", func() {
		fakeClient = newClient(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName, Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{Name: "kube-apiserver", Protocol: corev1.ProtocolTCP, Port: 6443, TargetPort: intstr.FromInt32(6443), NodePort: 30147},
					{Name: "talos-trustd", Protocol: corev1.ProtocolTCP, AppProtocol: ptr.To("grpc"), Port: additionalPort, TargetPort: intstr.FromInt32(additionalPort), NodePort: additionalNodePort},
				},
			},
		})
		tcp.Spec.ControlPlane.Service.AdditionalPorts[0].AppProtocol = ptr.To("grpc")

		reconcile()

		svc := getService()
		Expect(portNames(svc)).To(Equal([]string{"kube-apiserver", "konnectivity-server", "talos-trustd"}))
		Expect(svc.Spec.Ports[1].Port).To(Equal(konnectivityPort))
		Expect(svc.Spec.Ports[1].AppProtocol).To(BeNil(), "the konnectivity port must not inherit fields of the additional port")
		Expect(svc.Spec.Ports[1].NodePort).To(BeZero())
		Expect(svc.Spec.Ports[2].AppProtocol).To(Equal(ptr.To("grpc")))
	})

	It("updates the konnectivity port by name and reports it in the status", func() {
		fakeClient = newClient(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName, Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{Name: "kube-apiserver", Protocol: corev1.ProtocolTCP, Port: 6443, TargetPort: intstr.FromInt32(6443)},
					{Name: "talos-trustd", Protocol: corev1.ProtocolTCP, Port: additionalPort, TargetPort: intstr.FromInt32(additionalPort)},
					{Name: "konnectivity-server", Protocol: corev1.ProtocolTCP, Port: 8131, TargetPort: intstr.FromInt32(8131)},
				},
			},
		})

		konn := &konnectivity.ServiceResource{Client: fakeClient}
		Expect(konn.Define(ctx, tcp)).To(Succeed())
		_, err := konn.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		Expect(portNames(getService())).To(Equal([]string{"kube-apiserver", "talos-trustd", "konnectivity-server"}))
		Expect(getService().Spec.Ports[2].Port).To(Equal(konnectivityPort))

		Expect(konn.ShouldStatusBeUpdated(ctx, tcp)).To(BeTrue())
		Expect(konn.UpdateTenantControlPlaneStatus(ctx, tcp)).To(Succeed())
		Expect(tcp.Status.Addons.Konnectivity.Service.Port).To(Equal(konnectivityPort))
		Expect(konn.ShouldStatusBeUpdated(ctx, tcp)).To(BeFalse())
	})
})
