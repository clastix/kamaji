// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"testing"

	"k8s.io/utils/ptr"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestGetControlPlaneAddressAndPortFromGateway(t *testing.T) {
	testCases := []struct {
		name        string
		hostname    v1.Hostname
		parentRefs  []v1.ParentReference
		defaultPort int32
		wantAddr    string
		wantPort    int32
	}{
		{
			name:        "use provided port from first parent reference",
			hostname:    v1.Hostname("gateway.example.com"),
			parentRefs:  []v1.ParentReference{{Port: ptr.To(v1.PortNumber(443))}},
			defaultPort: 6443,
			wantAddr:    "gateway.example.com",
			wantPort:    443,
		},
		{
			name:        "no parent reference, use default port",
			hostname:    v1.Hostname("gateway.example.com"),
			parentRefs:  []v1.ParentReference{{}},
			defaultPort: 6443,
			wantAddr:    "gateway.example.com",
			wantPort:    6443,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotAddr, gotPort := GetControlPlaneAddressAndPortFromGateway(tt.hostname, tt.parentRefs, tt.defaultPort)
			if gotAddr != tt.wantAddr {
				t.Errorf("expected address %q, got %q", tt.wantAddr, gotAddr)
			}
			if gotPort != tt.wantPort {
				t.Errorf("expected port %q, got %q", tt.wantPort, gotPort)
			}
		})
	}
}
