// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities_test

import (
	"testing"

	"github.com/clastix/kamaji/internal/utilities"
)

func TestGetControlPlaneAddressAndPortFromHostname(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hostname string
		address  string
		port     int32
	}{
		{"foo.example.com", "foo.example.com", 6443},
		{"foo.example.com:8443", "foo.example.com", 8443},
		{"foo.example.com:abc", "foo.example.com", 6443},
		{"2001:db8::1", "2001:db8::1", 6443},
		{"[2001:db8::1]", "2001:db8::1", 6443},
		{"[2001:db8::1]:8443", "2001:db8::1", 8443},
	}

	for _, tc := range tests {
		address, port := utilities.GetControlPlaneAddressAndPortFromHostname(tc.hostname, 6443)
		if address != tc.address || port != tc.port {
			t.Errorf("%q: got (%q, %d), want (%q, %d)", tc.hostname, address, port, tc.address, tc.port)
		}
	}
}
