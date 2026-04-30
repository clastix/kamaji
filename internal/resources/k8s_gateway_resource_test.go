// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

func TestGatewayResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateway Resource Suite")
}

var runtimeScheme *runtime.Scheme

var _ = BeforeSuite(func() {
	runtimeScheme = runtime.NewScheme()
	Expect(scheme.AddToScheme(runtimeScheme)).To(Succeed())
	Expect(kamajiv1alpha1.AddToScheme(runtimeScheme)).To(Succeed())
	Expect(gatewayv1.Install(runtimeScheme)).To(Succeed())
	Expect(gatewayv1alpha2.Install(runtimeScheme)).To(Succeed())
})

var _ = Describe("KubernetesGatewayResource", func() {
	var (
		tcp      *kamajiv1alpha1.TenantControlPlane
		resource *resources.KubernetesGatewayResource
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient := fake.NewClientBuilder().
			WithScheme(runtimeScheme).
			Build()

		resource = &resources.KubernetesGatewayResource{
			Client: fakeClient,
		}

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Gateway: &kamajiv1alpha1.GatewaySpec{
						Hostname: gatewayv1alpha2.Hostname("test.example.com"),
						AdditionalMetadata: kamajiv1alpha1.AdditionalMetadata{
							Labels: map[string]string{
								"test-label": "test-value",
							},
						},
						GatewayParentRefs: []gatewayv1alpha2.ParentReference{
							{
								Name: "test-gateway",
							},
						},
					},
				},
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Kubernetes: kamajiv1alpha1.KubernetesStatus{
					Service: kamajiv1alpha1.KubernetesServiceStatus{
						Name: "test-service",
						Port: 6443,
					},
				},
			},
		}
	})

	Context("When GatewayRoutes is configured", func() {
		It("should not cleanup", func() {
			Expect(resource.ShouldCleanup(tcp)).To(BeFalse())
		})

		It("should define route resources", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should require status update when GatewayRoutes is configured but status is nil", func() {
			tcp.Status.Kubernetes.Gateway = nil
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeTrue())
		})

		It("should handle multiple parentRefs correctly", func() {
			namespace := gatewayv1.Namespace("default")
			tcp.Spec.ControlPlane.Gateway.GatewayParentRefs = []gatewayv1alpha2.ParentReference{
				{
					Name:      "test-gateway-1",
					Namespace: &namespace,
				},
				{
					Name:      "test-gateway-2",
					Namespace: &namespace,
				},
			}

			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			route := &gatewayv1alpha2.TLSRoute{}
			err = resource.Client.Get(ctx, client.ObjectKey{Name: tcp.Name, Namespace: tcp.Namespace}, route)
			Expect(err).NotTo(HaveOccurred())
			Expect(route.Spec.ParentRefs).To(HaveLen(2))
			Expect(route.Spec.ParentRefs[0].Name).To(Equal(gatewayv1alpha2.ObjectName("test-gateway-1")))
			Expect(route.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
			Expect(*route.Spec.ParentRefs[0].Namespace).To(Equal(namespace))
			Expect(route.Spec.ParentRefs[1].Name).To(Equal(gatewayv1alpha2.ObjectName("test-gateway-2")))
			Expect(route.Spec.ParentRefs[1].Namespace).NotTo(BeNil())
			Expect(*route.Spec.ParentRefs[1].Namespace).To(Equal(namespace))
		})
	})

	Context("When GatewayRoutes is not configured", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Gateway = nil
			tcp.Status.Kubernetes.Gateway = &kamajiv1alpha1.KubernetesGatewayStatus{
				AccessPoints: nil,
			}
		})

		It("should cleanup", func() {
			Expect(resource.ShouldCleanup(tcp)).To(BeTrue())
		})

		It("should not require status update when both spec and status are nil", func() {
			tcp.Status.Kubernetes.Gateway = nil
			shouldUpdate := resource.ShouldStatusBeUpdated(ctx, tcp)
			Expect(shouldUpdate).To(BeFalse())
		})
	})

	Context("When hostname is missing", func() {
		BeforeEach(func() {
			tcp.Spec.ControlPlane.Gateway.Hostname = ""
		})

		It("should fail to create or update", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing hostname"))
		})
	})

	Context("When service is not ready", func() {
		BeforeEach(func() {
			tcp.Status.Kubernetes.Service.Name = ""
			tcp.Status.Kubernetes.Service.Port = 0
		})

		It("should fail to create or update", func() {
			err := resource.Define(ctx, tcp)
			Expect(err).NotTo(HaveOccurred())

			_, err = resource.CreateOrUpdate(ctx, tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("service not ready"))
		})
	})

	It("should return correct resource name", func() {
		Expect(resource.GetName()).To(Equal("gateway_routes"))
	})

	Describe("findMatchingListener", func() {
		var (
			listeners []gatewayv1.Listener
			ref       gatewayv1.ParentReference
		)

		BeforeEach(func() {
			listeners = []gatewayv1.Listener{
				{
					Name: "first",
					Port: gatewayv1.PortNumber(443),
				},
				{
					Name: "middle",
					Port: gatewayv1.PortNumber(6443),
				},
				{
					Name: "last",
					Port: gatewayv1.PortNumber(80),
				},
			}
			ref = gatewayv1.ParentReference{
				Name: "test-gateway",
			}
		})

		It("should return an error when sectionName is nil", func() {
			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing sectionName"))
			Expect(listener).To(Equal(gatewayv1.Listener{}))
		})
		It("should return an error when sectionName is an empty string", func() {
			sectionName := gatewayv1.SectionName("")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find listener ''"))
			Expect(listener).To(Equal(gatewayv1.Listener{}))
		})

		It("should return the matching listener when sectionName points to an existing listener", func() {
			sectionName := gatewayv1.SectionName("middle")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(listener.Port).To(Equal(gatewayv1.PortNumber(6443)))
		})

		It("should return an error when sectionName points to a non-existent listener", func() {
			sectionName := gatewayv1.SectionName("non-existent")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find listener 'non-existent'"))
			Expect(listener).To(Equal(gatewayv1.Listener{}))
		})

		It("should return the first listener", func() {
			sectionName := gatewayv1.SectionName("first")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(listener.Port).To(Equal(gatewayv1.PortNumber(443)))
		})

		It("should return the last listener when matching by name", func() {
			sectionName := gatewayv1.SectionName("last")
			ref.SectionName = &sectionName

			listener, err := resources.FindMatchingListener(listeners, ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(listener.Port).To(Equal(gatewayv1.PortNumber(80)))
		})
	})

	Describe("BuildGatewayAccessPointsStatus", func() {
		var (
			gwNamespace = "gateway-system"
			gwName      = "test-gateway"
			gateway     *gatewayv1.Gateway
			route       *gatewayv1alpha2.TLSRoute
			fakeClient  client.Client
		)

		// Builds a RouteStatus with a single Accepted parent using the
		// supplied ParentReference.
		buildRouteStatus := func(ref gatewayv1.ParentReference) gatewayv1alpha2.RouteStatus {
			return gatewayv1alpha2.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{{
					ParentRef: ref,
					Conditions: []metav1.Condition{{
						Type:               string(gatewayv1.RouteConditionAccepted),
						Status:             metav1.ConditionTrue,
						Reason:             "Accepted",
						LastTransitionTime: metav1.Now(),
					}},
				}},
			}
		}

		BeforeEach(func() {
			gateway = &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: gwName, Namespace: gwNamespace},
				Spec: gatewayv1.GatewaySpec{
					Listeners: []gatewayv1.Listener{
						{Name: "kube-apiserver", Port: 31443, Protocol: gatewayv1.TLSProtocolType},
						{Name: "konnectivity-server", Port: 32132, Protocol: gatewayv1.TLSProtocolType},
						// Non-TLS listener on the same Gateway: it must not be turned
						// into a TLSRoute access point.
						{Name: "http-noise", Port: 8080, Protocol: gatewayv1.HTTPProtocolType},
					},
				},
				Status: gatewayv1.GatewayStatus{
					Conditions: []metav1.Condition{{
						Type:               string(gatewayv1.GatewayConditionProgrammed),
						Status:             metav1.ConditionTrue,
						Reason:             "Programmed",
						LastTransitionTime: metav1.Now(),
					}},
				},
			}

			route = &gatewayv1alpha2.TLSRoute{
				ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "tenant-ns"},
				Spec: gatewayv1alpha2.TLSRouteSpec{
					Hostnames: []gatewayv1.Hostname{"tcp.example.com"},
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(runtimeScheme).
				WithObjects(gateway).
				WithIndex(&gatewayv1.Gateway{}, kamajiv1alpha1.GatewayListenerNameKey, (&kamajiv1alpha1.GatewayListener{}).ExtractValue()).
				Build()
		})

		It("builds a single access point when the parentRef specifies a sectionName", func() {
			section := gatewayv1.SectionName("konnectivity-server")
			ns := gatewayv1.Namespace(gwNamespace)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:        gatewayv1.ObjectName(gwName),
				Namespace:   &ns,
				SectionName: &section,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(HaveLen(1))
			Expect(aps[0].Port).To(Equal(gatewayv1.PortNumber(32132)))
			Expect(aps[0].Value).To(Equal("https://tcp.example.com:32132"))
		})

		It("ignores a non-TLS listener even when explicitly selected via sectionName", func() {
			section := gatewayv1.SectionName("http-noise")
			ns := gatewayv1.Namespace(gwNamespace)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:        gatewayv1.ObjectName(gwName),
				Namespace:   &ns,
				SectionName: &section,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(BeEmpty())
		})

		It("ignores a TLS listener whose port disagrees with parentRef.Port", func() {
			section := gatewayv1.SectionName("kube-apiserver")
			ns := gatewayv1.Namespace(gwNamespace)
			mismatch := gatewayv1.PortNumber(9999)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:        gatewayv1.ObjectName(gwName),
				Namespace:   &ns,
				SectionName: &section,
				Port:        &mismatch,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(BeEmpty())
		})

		It("builds one access point per listener when sectionName is unset", func() {
			ns := gatewayv1.Namespace(gwNamespace)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:      gatewayv1.ObjectName(gwName),
				Namespace: &ns,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(HaveLen(2))
			ports := []gatewayv1.PortNumber{aps[0].Port, aps[1].Port}
			Expect(ports).To(ConsistOf(gatewayv1.PortNumber(31443), gatewayv1.PortNumber(32132)))
		})

		It("filters listeners by port when sectionName is unset but port is specified", func() {
			ns := gatewayv1.Namespace(gwNamespace)
			port := gatewayv1.PortNumber(31443)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:      gatewayv1.ObjectName(gwName),
				Namespace: &ns,
				Port:      &port,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(HaveLen(1))
			Expect(aps[0].Port).To(Equal(port))
		})

		It("defaults the parent namespace to the route namespace when unset", func() {
			// Gateway is in the route's namespace so the nil-namespace default kicks in.
			colocated := gateway.DeepCopy()
			colocated.Namespace = route.Namespace

			c := fake.NewClientBuilder().
				WithScheme(runtimeScheme).
				WithObjects(colocated).
				WithIndex(&gatewayv1.Gateway{}, kamajiv1alpha1.GatewayListenerNameKey, (&kamajiv1alpha1.GatewayListener{}).ExtractValue()).
				Build()

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name: gatewayv1.ObjectName(gwName),
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, c, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(HaveLen(2))
		})

		It("skips silently when the referenced Gateway is missing", func() {
			ns := gatewayv1.Namespace("does-not-exist")

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:      gatewayv1.ObjectName("ghost-gateway"),
				Namespace: &ns,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(BeEmpty())
		})

		It("skips Gateways that are not Programmed", func() {
			notProgrammed := gateway.DeepCopy()
			notProgrammed.Status.Conditions = nil

			c := fake.NewClientBuilder().
				WithScheme(runtimeScheme).
				WithObjects(notProgrammed).
				WithIndex(&gatewayv1.Gateway{}, kamajiv1alpha1.GatewayListenerNameKey, (&kamajiv1alpha1.GatewayListener{}).ExtractValue()).
				Build()

			ns := gatewayv1.Namespace(gwNamespace)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:      gatewayv1.ObjectName(gwName),
				Namespace: &ns,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, c, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			Expect(aps).To(BeEmpty())
		})

		It("ignores non-TLS listeners when sectionName is unset", func() {
			ns := gatewayv1.Namespace(gwNamespace)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:      gatewayv1.ObjectName(gwName),
				Namespace: &ns,
			})

			aps, err := resources.BuildGatewayAccessPointsStatus(ctx, fakeClient, route, statuses)
			Expect(err).NotTo(HaveOccurred())
			// Only the two TLS listeners may produce https:// access points.
			Expect(aps).To(HaveLen(2))
			ports := []gatewayv1.PortNumber{aps[0].Port, aps[1].Port}
			Expect(ports).To(ConsistOf(gatewayv1.PortNumber(31443), gatewayv1.PortNumber(32132)))
			Expect(ports).NotTo(ContainElement(gatewayv1.PortNumber(8080)))
		})

		It("propagates indexer failures from the sectionName fast path", func() {
			// Fault-injection; build a client *without* the listener-name, i.e.
			// WithIndex(&gatewayv1.Gateway{}, GatewayListenerNameKey, …).
			// An error is expected to be propagated
			c := fake.NewClientBuilder().
				WithScheme(runtimeScheme).
				WithObjects(gateway).
				Build()

			section := gatewayv1.SectionName("konnectivity-server")
			ns := gatewayv1.Namespace(gwNamespace)

			statuses := buildRouteStatus(gatewayv1.ParentReference{
				Name:        gatewayv1.ObjectName(gwName),
				Namespace:   &ns,
				SectionName: &section,
			})

			_, err := resources.BuildGatewayAccessPointsStatus(ctx, c, route, statuses)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not resolve gateway listeners for parentRef"))
			Expect(err.Error()).To(ContainSubstring("failed to fetch gateway for listener"))
		})
	})
})
