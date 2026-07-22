// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestControlPlaneServiceIPs(t *testing.T) {
	const (
		name = "tcp"
		ns   = "default"
	)

	tcp := &TenantControlPlane{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}

	svc := func(mutate func(*corev1.Service)) *corev1.Service {
		s := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
		mutate(s)

		return s
	}

	tests := []struct {
		name    string
		service *corev1.Service
		want    []string
	}{
		{
			name: "dual-stack ClusterIP returns both families",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeClusterIP
				s.Spec.ClusterIP = "10.96.0.1"
				s.Spec.ClusterIPs = []string{"10.96.0.1", "2001:db8::1"}
			}),
			want: []string{"10.96.0.1", "2001:db8::1"},
		},
		{
			name: "single-stack ClusterIP returns one IP",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeClusterIP
				s.Spec.ClusterIP = "2001:db8::1"
				s.Spec.ClusterIPs = []string{"2001:db8::1"}
			}),
			want: []string{"2001:db8::1"},
		},
		{
			name: "legacy Service with only ClusterIP falls back",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeClusterIP
				s.Spec.ClusterIP = "10.96.0.1"
			}),
			want: []string{"10.96.0.1"},
		},
		{
			name: "headless ClusterIP is skipped",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeClusterIP
				s.Spec.ClusterIP = corev1.ClusterIPNone
				s.Spec.ClusterIPs = []string{corev1.ClusterIPNone}
			}),
			want: []string{},
		},
		{
			name: "LoadBalancer returns ClusterIPs and all ingress IPs",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeLoadBalancer
				s.Spec.ClusterIP = "10.96.0.1"
				s.Spec.ClusterIPs = []string{"10.96.0.1", "2001:db8::1"}
				s.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{IP: "192.0.2.10"},
					{IP: "2001:db8:cafe::10"},
				}
			}),
			want: []string{"10.96.0.1", "2001:db8::1", "192.0.2.10", "2001:db8:cafe::10"},
		},
		{
			name: "LoadBalancer not yet provisioned returns ClusterIPs only",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeLoadBalancer
				s.Spec.ClusterIP = "10.96.0.1"
				s.Spec.ClusterIPs = []string{"10.96.0.1"}
			}),
			want: []string{"10.96.0.1"},
		},
		{
			name: "ingress hostname without IP is skipped",
			service: svc(func(s *corev1.Service) {
				s.Spec.Type = corev1.ServiceTypeLoadBalancer
				s.Spec.ClusterIP = "10.96.0.1"
				s.Spec.ClusterIPs = []string{"10.96.0.1"}
				s.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: "lb.example.com"}}
			}),
			want: []string{"10.96.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithObjects(tt.service).Build()

			got, err := tcp.ControlPlaneServiceIPs(context.Background(), c)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestControlPlaneServiceIPsMissingService(t *testing.T) {
	tcp := &TenantControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "default"}}
	c := fake.NewClientBuilder().Build()

	if _, err := tcp.ControlPlaneServiceIPs(context.Background(), c); err == nil {
		t.Fatal("expected an error when the Service does not exist, got nil")
	}
}
