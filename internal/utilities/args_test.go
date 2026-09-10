// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"maps"
	"slices"
	"testing"
)

func TestArgsFromSliceToMap(t *testing.T) {
	tests := map[string]map[string]string{
		"--a":     {"--a": ""},
		"--a=":    {"--a": ""},
		"--a=b":   {"--a": "b"},
		"--a=b=c": {"--a": "b=c"},
	}

	got := ArgsFromSliceToMap([]string{})
	if len(got) != 0 {
		t.Errorf("expected empty input to result in empty map, but got %+v", got)
	}

	for arg, expect := range tests {
		got := ArgsFromSliceToMap([]string{arg})
		if !maps.Equal(expect, got) {
			t.Errorf("expected input %q to result in %+v, but got %+v", arg, expect, got)
		}
	}
}

func TestInsertArgInSortedSegment(t *testing.T) {
	const egress = "--egress-selector-config-file=/etc/kubernetes/konnectivity/egress.yaml"

	tests := map[string]struct {
		args       []string
		userExtras []string
		expect     []string
	}{
		"empty args": {
			args:   nil,
			expect: []string{egress},
		},
		"sorts into a Kamaji-owned only list": {
			args:   []string{"--advertise-address=10.0.0.1", "--secure-port=6443"},
			expect: []string{"--advertise-address=10.0.0.1", egress, "--secure-port=6443"},
		},
		"keeps user extras at the end": {
			args:       []string{"--advertise-address=10.0.0.1", "--secure-port=6443", "--audit-log-path=/x"},
			userExtras: []string{"--audit-log-path=/x"},
			expect:     []string{"--advertise-address=10.0.0.1", egress, "--secure-port=6443", "--audit-log-path=/x"},
		},
		"does not sort into user extras that continue the sorted run": {
			args:       []string{"--advertise-address=10.0.0.1", "--zzz=1"},
			userExtras: []string{"--zzz=1"},
			expect:     []string{"--advertise-address=10.0.0.1", egress, "--zzz=1"},
		},
		"handles repeated user extras": {
			args: []string{
				"--advertise-address=10.0.0.1",
				"--service-account-issuer=https://one.example.com",
				"--service-account-issuer=https://two.example.com",
			},
			userExtras: []string{
				"--service-account-issuer=https://one.example.com",
				"--service-account-issuer=https://two.example.com",
			},
			expect: []string{
				"--advertise-address=10.0.0.1",
				egress,
				"--service-account-issuer=https://one.example.com",
				"--service-account-issuer=https://two.example.com",
			},
		},
		"skips user extras the merge dropped": {
			args:       []string{"--advertise-address=10.0.0.1", "--audit-log-path=/x"},
			userExtras: []string{"--advertise-address=192.168.0.1", "--audit-log-path=/x"},
			expect:     []string{"--advertise-address=10.0.0.1", egress, "--audit-log-path=/x"},
		},
	}

	for name, tc := range tests {
		got := InsertArgInSortedSegment(tc.args, tc.userExtras, egress)
		if !slices.Equal(got, tc.expect) {
			t.Errorf("%s: expected %+v, but got %+v", name, tc.expect, got)
		}
	}
}
