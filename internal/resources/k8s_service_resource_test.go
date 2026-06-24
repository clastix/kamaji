// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

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

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

var _ = Describe("KubernetesServiceResource AllocateLoadBalancerNodePorts", func() {
	var (
		ctx context.Context
		tcp *kamajiv1alpha1.TenantControlPlane
	)

	// tcpName is shared by both the TenantControlPlane ObjectMeta and the existingService
	// fixture so they can't silently drift apart (Define() derives the Service name from
	// the TCP name, so they must match).
	const tcpName = "test-tcp"

	// seededNodePort is an arbitrary fixed value in the NodePort range that we
	// pre-populate on the Service fixture. The fake client never allocates NodePorts,
	// so this value is fully deterministic in the test — unlike a real cluster, where
	// Kubernetes assigns one at random from 30000-32767.
	const seededNodePort int32 = 30654

	// existingService mimics an already-reconciled LoadBalancer Service that
	// already has a NodePort assigned (as Kubernetes would allocate by default).
	existingService := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName, Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{{
					Name:       "kube-apiserver",
					Protocol:   corev1.ProtocolTCP,
					Port:       6443,
					TargetPort: intstr.FromInt32(6443),
					NodePort:   seededNodePort,
				}},
			},
		}
	}

	newResource := func(objs ...client.Object) *resources.KubernetesServiceResource {
		fakeClient := fake.NewClientBuilder().
			WithScheme(runtimeScheme).
			WithObjects(objs...).
			Build()

		return &resources.KubernetesServiceResource{Client: fakeClient}
	}

	BeforeEach(func() {
		ctx = context.Background()
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName, Namespace: "default"},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer,
					},
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					Port: 6443,
				},
			},
		}
	})

	It("disables allocation and clears an existing NodePort when set to false", func() {
		tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = ptr.To(false)
		resource := newResource(existingService())

		Expect(resource.Define(ctx, tcp)).To(Succeed())
		_, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		svc := &corev1.Service{}
		Expect(resource.Client.Get(ctx, client.ObjectKey{Name: tcp.Name, Namespace: tcp.Namespace}, svc)).To(Succeed())
		Expect(svc.Spec.AllocateLoadBalancerNodePorts).NotTo(BeNil())
		Expect(*svc.Spec.AllocateLoadBalancerNodePorts).To(BeFalse())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].NodePort).To(BeZero())
	})

	It("defaults to true when unset, preserving an existing NodePort", func() {
		// An unset (nil) field means "use the Kubernetes LoadBalancer default" (true).
		// The builder writes true explicitly so clearing the field reverts the Service to
		// the default without churn.
		tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = nil
		resource := newResource(existingService())

		Expect(resource.Define(ctx, tcp)).To(Succeed())
		_, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		svc := &corev1.Service{}
		Expect(resource.Client.Get(ctx, client.ObjectKey{Name: tcp.Name, Namespace: tcp.Namespace}, svc)).To(Succeed())
		Expect(svc.Spec.AllocateLoadBalancerNodePorts).NotTo(BeNil())
		Expect(*svc.Spec.AllocateLoadBalancerNodePorts).To(BeTrue())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].NodePort).To(Equal(seededNodePort))
	})

	It("does not churn against a server-defaulted true when field is unset", func() {
		// Writing the same default value (true) that the API server already defaulted to
		// produces no diff on DeepEqual, preventing perpetual reconcile churn.
		tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = nil
		existing := existingService()
		existing.Spec.AllocateLoadBalancerNodePorts = ptr.To(true) // API server default on a live LB Service
		resource := newResource(existing)

		Expect(resource.Define(ctx, tcp)).To(Succeed())
		_, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		svc := &corev1.Service{}
		Expect(resource.Client.Get(ctx, client.ObjectKey{Name: tcp.Name, Namespace: tcp.Namespace}, svc)).To(Succeed())
		Expect(svc.Spec.AllocateLoadBalancerNodePorts).NotTo(BeNil())
		Expect(*svc.Spec.AllocateLoadBalancerNodePorts).To(BeTrue())
	})

	It("reverts a previously-false allocation to the default when field is cleared (unset)", func() {
		// Clearing the field (setting to nil in the TCP spec) is the declarative way to
		// revert to the Kubernetes LoadBalancer default (true).
		tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = nil
		existing := existingService()
		existing.Spec.AllocateLoadBalancerNodePorts = ptr.To(false) // previously disabled
		resource := newResource(existing)

		Expect(resource.Define(ctx, tcp)).To(Succeed())
		_, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		svc := &corev1.Service{}
		Expect(resource.Client.Get(ctx, client.ObjectKey{Name: tcp.Name, Namespace: tcp.Namespace}, svc)).To(Succeed())
		Expect(svc.Spec.AllocateLoadBalancerNodePorts).NotTo(BeNil())
		Expect(*svc.Spec.AllocateLoadBalancerNodePorts).To(BeTrue())
		// NodePort re-allocation is not modeled by the fake client; no port assertion here.
	})

	It("propagates an explicit true and leaves the NodePort untouched", func() {
		tcp.Spec.ControlPlane.Service.AllocateLoadBalancerNodePorts = ptr.To(true)
		resource := newResource(existingService())

		Expect(resource.Define(ctx, tcp)).To(Succeed())
		_, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		svc := &corev1.Service{}
		Expect(resource.Client.Get(ctx, client.ObjectKey{Name: tcp.Name, Namespace: tcp.Namespace}, svc)).To(Succeed())
		Expect(svc.Spec.AllocateLoadBalancerNodePorts).NotTo(BeNil())
		Expect(*svc.Spec.AllocateLoadBalancerNodePorts).To(BeTrue())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].NodePort).To(Equal(seededNodePort))
	})
})
