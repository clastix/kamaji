// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"slices"
	"testing"
)

func TestGetEffectiveCIDRsPreservesOrder(t *testing.T) {
	tests := []struct {
		name       string
		deprecated string
		current    []string
		expect     []string
	}{
		// The primary IP family of a dual-stack cluster is decided by the FIRST
		// CIDR, so both orders must survive verbatim.
		{
			name:    "IPv6-primary dual-stack is preserved",
			current: []string{"fd00:96::/108", "10.96.0.0/16"},
			expect:  []string{"fd00:96::/108", "10.96.0.0/16"},
		},
		{
			name:    "IPv4-primary dual-stack is preserved",
			current: []string{"10.96.0.0/16", "fd00:96::/108"},
			expect:  []string{"10.96.0.0/16", "fd00:96::/108"},
		},
		// A lexicographic sort would move the IPv4 entry first here.
		{
			name:    "IPv6-primary is not reordered by lexicographic comparison",
			current: []string{"fd00:244::/56", "10.244.0.0/16"},
			expect:  []string{"fd00:244::/56", "10.244.0.0/16"},
		},
		// The input is already in lexicographic order, so a sorting
		// implementation and an order-preserving one agree on the order and the
		// only thing this case can fail on is WHICH occurrence is kept: keeping
		// the last one would yield ["fd00:96::/108", "10.96.0.0/16"].
		{
			name:    "duplicates are removed keeping the first occurrence",
			current: []string{"10.96.0.0/16", "fd00:96::/108", "10.96.0.0/16"},
			expect:  []string{"10.96.0.0/16", "fd00:96::/108"},
		},
		{
			name:    "duplicates are removed and the order is preserved",
			current: []string{"fd00:96::/108", "10.96.0.0/16", "fd00:96::/108"},
			expect:  []string{"fd00:96::/108", "10.96.0.0/16"},
		},
		{
			name:    "single stack is preserved",
			current: []string{"fd00:96::/108"},
			expect:  []string{"fd00:96::/108"},
		},
		{
			name:       "the plural field takes precedence over the deprecated one",
			deprecated: "10.96.0.0/16",
			current:    []string{"fd00:96::/108", "10.96.0.0/16"},
			expect:     []string{"fd00:96::/108", "10.96.0.0/16"},
		},
		{
			name:       "the deprecated field is the fallback",
			deprecated: "10.96.0.0/16",
			expect:     []string{"10.96.0.0/16"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetEffectiveCIDRs(tc.deprecated, tc.current)
			if !slices.Equal(got, tc.expect) {
				t.Errorf("expected %+v, but got %+v", tc.expect, got)
			}
		})
	}
}

func TestGetEffectiveCIDRsNoCIDRsYieldsNil(t *testing.T) {
	// slices.Equal(nil, []string{}) is true, so the nil-ness has to be asserted
	// explicitly rather than through a comparison with an expected value.
	if got := GetEffectiveCIDRs("", nil); got != nil {
		t.Errorf("expected nil, but got %#v", got)
	}

	if got := GetEffectiveCIDRs("", []string{}); got != nil {
		t.Errorf("expected nil for an empty slice, but got %#v", got)
	}
}

func TestGetEffectiveCIDRsDoesNotMutateInput(t *testing.T) {
	current := []string{"fd00:96::/108", "10.96.0.0/16"}
	original := slices.Clone(current)

	GetEffectiveCIDRs("", current)

	if !slices.Equal(current, original) {
		t.Errorf("expected the input slice to be untouched, but it became %+v", current)
	}
}
