// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestResolveTenantControlPlaneAddressGateway(t *testing.T) {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{Port: 6443},
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Gateway: &kamajiv1alpha1.GatewaySpec{Hostname: "tcp1.cluster.dev"},
			},
		},
	}

	address := resolveGatewayAddress(tcp)
	if address != "https://tcp1.cluster.dev:6443" {
		t.Fatalf("expected gateway address https://tcp1.cluster.dev:6443, got %q", address)
	}
}

func TestResolveTenantControlPlaneAddressServicePrefersIP(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 6443}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{Hostname: "api.cluster.dev"}, {IP: "203.0.113.10"}},
			},
		},
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(service).Build()
	tcp := &kamajiv1alpha1.TenantControlPlane{
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Service: kamajiv1alpha1.ServiceSpec{ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer},
			},
		},
		Status: kamajiv1alpha1.TenantControlPlaneStatus{
			Kubernetes: kamajiv1alpha1.KubernetesStatus{
				Service: kamajiv1alpha1.KubernetesServiceStatus{
					Name:      "tcp-svc",
					Namespace: "default",
					Port:      6443,
				},
			},
		},
	}

	address := resolveTenantControlPlaneAddress(t.Context(), reader, tcp)
	if address != "https://203.0.113.10:6443" {
		t.Fatalf("expected service address https://203.0.113.10:6443, got %q", address)
	}
}

func TestResolveTenantControlPlaneAddressServiceFallbackToHostname(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 6443}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{Hostname: "api.cluster.dev"}},
			},
		},
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(service).Build()
	tcp := &kamajiv1alpha1.TenantControlPlane{
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Service: kamajiv1alpha1.ServiceSpec{ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer},
			},
		},
		Status: kamajiv1alpha1.TenantControlPlaneStatus{
			Kubernetes: kamajiv1alpha1.KubernetesStatus{
				Service: kamajiv1alpha1.KubernetesServiceStatus{
					Name:      "tcp-svc",
					Namespace: "default",
					Port:      6443,
				},
			},
		},
	}

	address := resolveTenantControlPlaneAddress(t.Context(), reader, tcp)
	if address != "https://api.cluster.dev:6443" {
		t.Fatalf("expected service address https://api.cluster.dev:6443, got %q", address)
	}
}

func TestResolveTenantControlPlaneAddressReturnsEmptyWhenUnresolvable(t *testing.T) {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Service: kamajiv1alpha1.ServiceSpec{ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer},
			},
		},
	}

	reader := fake.NewClientBuilder().Build()
	address := resolveTenantControlPlaneAddress(t.Context(), reader, tcp)
	if address != "" {
		t.Fatalf("expected empty address fallback, got %q", address)
	}
}
