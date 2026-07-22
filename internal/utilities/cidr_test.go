// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities_test

import (
	"reflect"
	"testing"

	"github.com/clastix/kamaji/internal/utilities"
)

func TestGetEffectiveCIDRs(t *testing.T) {
	tests := []struct {
		name       string
		deprecated string
		current    []string
		want       []string
	}{
		{
			name:    "preserves IPv6-first user order",
			current: []string{"fd00::/120", "10.96.0.0/16"},
			want:    []string{"fd00::/120", "10.96.0.0/16"},
		},
		{
			name:    "preserves IPv4-first user order",
			current: []string{"10.96.0.0/16", "fd00::/120"},
			want:    []string{"10.96.0.0/16", "fd00::/120"},
		},
		{
			name:    "removes duplicates keeping first occurrence order",
			current: []string{"fd00::/120", "10.96.0.0/16", "fd00::/120"},
			want:    []string{"fd00::/120", "10.96.0.0/16"},
		},
		{
			name:       "falls back to the deprecated single CIDR",
			deprecated: "10.96.0.0/16",
			want:       []string{"10.96.0.0/16"},
		},
		{
			name:       "current takes precedence over deprecated",
			deprecated: "10.0.0.0/16",
			current:    []string{"fd00::/120"},
			want:       []string{"fd00::/120"},
		},
		{
			name: "nil when nothing is set",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utilities.GetEffectiveCIDRs(tt.deprecated, tt.current)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("GetEffectiveCIDRs(%q, %v) = %v, want %v", tt.deprecated, tt.current, got, tt.want)
			}
		})
	}
}
