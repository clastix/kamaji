// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"maps"
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
