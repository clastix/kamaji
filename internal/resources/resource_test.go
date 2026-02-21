// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestStripLoadBalancerPortsFromServiceStatus(t *testing.T) {
	ipModeProxy := corev1.LoadBalancerIPModeProxy

	tests := []struct {
		name   string
		input  corev1.ServiceStatus
		assert func(t *testing.T, orig, got corev1.ServiceStatus)
	}{
		{
			name: "ip ingress with ports and ipMode",
			input: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP:     "172.18.0.3",
							IPMode: &ipModeProxy,
							Ports: []corev1.PortStatus{
								{Port: 6443, Protocol: corev1.ProtocolTCP},
								{Port: 8132, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			assert: func(t *testing.T, orig, got corev1.ServiceStatus) {
				t.Helper()
				if got.LoadBalancer.Ingress[0].Ports != nil {
					t.Fatalf("expected ports stripped, got %#v", got.LoadBalancer.Ingress[0].Ports)
				}
				if got.LoadBalancer.Ingress[0].IP != "172.18.0.3" {
					t.Fatalf("IP not preserved")
				}
				if got.LoadBalancer.Ingress[0].IPMode == nil || *got.LoadBalancer.Ingress[0].IPMode != ipModeProxy {
					t.Fatalf("IPMode not preserved")
				}
				if orig.LoadBalancer.Ingress[0].Ports == nil {
					t.Fatalf("original ports mutated")
				}
			},
		},
		{
			name: "hostname ingress with ports",
			input: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: "example.local",
							Ports: []corev1.PortStatus{
								{Port: 6443, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			assert: func(t *testing.T, orig, got corev1.ServiceStatus) {
				t.Helper()
				if got.LoadBalancer.Ingress[0].Ports != nil {
					t.Fatalf("expected ports stripped")
				}
				if got.LoadBalancer.Ingress[0].Hostname != "example.local" {
					t.Fatalf("hostname not preserved")
				}
			},
		},
		{
			name:  "no ingress",
			input: corev1.ServiceStatus{},
			assert: func(t *testing.T, _, got corev1.ServiceStatus) {
				t.Helper()
				if len(got.LoadBalancer.Ingress) != 0 {
					t.Fatalf("expected no ingress")
				}
			},
		},
		{
			name: "ingress with nil ports",
			input: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{IP: "10.0.0.1"},
					},
				},
			},
			assert: func(t *testing.T, _, got corev1.ServiceStatus) {
				t.Helper()
				if got.LoadBalancer.Ingress[0].Ports != nil {
					t.Fatalf("expected ports to stay nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := tt.input
			got := StripLoadBalancerPortsFromServiceStatus(tt.input)
			tt.assert(t, orig, got)
		})
	}
}
