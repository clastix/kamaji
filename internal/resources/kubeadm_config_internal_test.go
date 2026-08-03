// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"reflect"
	"testing"
)

func TestCanonicalSAN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "IPv4 unchanged", in: "10.96.0.1", want: "10.96.0.1"},
		{name: "compressed IPv6 unchanged", in: "2001:db8::1", want: "2001:db8::1"},
		{name: "expanded IPv6 collapses to canonical form", in: "2001:db8:0:0:0:0:0:1", want: "2001:db8::1"},
		{name: "uppercase IPv6 lowercased", in: "2001:DB8::1", want: "2001:db8::1"},
		{name: "IPv4-mapped IPv6 collapses to IPv4", in: "::ffff:10.96.0.1", want: "10.96.0.1"},
		{name: "DNS name passes through", in: "tcp.default.svc", want: "tcp.default.svc"},
		{name: "non-IP string passes through", in: "localhost", want: "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalSAN(tt.in); got != tt.want {
				t.Fatalf("canonicalSAN(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMergeCertSANs(t *testing.T) {
	tests := []struct {
		name       string
		management string
		certSANs   []string
		additional []string
		want       []string
	}{
		{
			name:       "single-stack adds nothing (service IP equals management address)",
			management: "10.96.0.1",
			certSANs:   []string{"127.0.0.1", "localhost", "tcp.default.svc"},
			additional: []string{"10.96.0.1"},
			want:       []string{"127.0.0.1", "localhost", "tcp.default.svc"},
		},
		{
			name:       "dual-stack appends the secondary family IP",
			management: "10.96.0.1",
			certSANs:   []string{"127.0.0.1", "localhost"},
			additional: []string{"10.96.0.1", "2001:db8::1"},
			want:       []string{"127.0.0.1", "localhost", "2001:db8::1"},
		},
		{
			name:       "secondary IP already present in a different textual form is not duplicated",
			management: "10.96.0.1",
			certSANs:   []string{"2001:db8:0:0:0:0:0:1"},
			additional: []string{"10.96.0.1", "2001:db8::1"},
			want:       []string{"2001:db8:0:0:0:0:0:1"},
		},
		{
			name:       "LoadBalancer ingress IPs are appended once",
			management: "192.0.2.10",
			certSANs:   []string{"192.0.2.10"},
			additional: []string{"10.96.0.1", "2001:db8::1", "192.0.2.10"},
			want:       []string{"192.0.2.10", "10.96.0.1", "2001:db8::1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeCertSANs(tt.management, tt.certSANs, tt.additional)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mergeCertSANs(%q, %v, %v) = %v, want %v", tt.management, tt.certSANs, tt.additional, got, tt.want)
			}
		})
	}
}
