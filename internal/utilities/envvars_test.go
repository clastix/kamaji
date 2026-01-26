// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"maps"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

type TestData struct {
	Slice []corev1.EnvVar
	Map   map[string]corev1.EnvVar
}

var testData map[string]TestData

func init() {
	testData = map[string]TestData{
		"empty slice": {
			Slice: []corev1.EnvVar{},
			Map:   map[string]corev1.EnvVar{},
		},

		"regular slice": {
			Slice: []corev1.EnvVar{
				{
					Name:  "var_0",
					Value: "value_0",
				},
				{
					Name:  "var_1",
					Value: "value_1",
				},
				{
					Name:  "var_2",
					Value: "value_2",
				},
			},
			Map: map[string]corev1.EnvVar{
				"var_2": {
					Name:  "var_2",
					Value: "value_2",
				},
				"var_0": {
					Name:  "var_0",
					Value: "value_0",
				},
				"var_1": {
					Name:  "var_1",
					Value: "value_1",
				},
			},
		},
	}
}

func TestEnvarsFromSliceToMap(t *testing.T) {
	for name, data := range testData {
		result := EnvarsFromSliceToMap(data.Slice)

		if !maps.Equal(data.Map, result) {
			t.Errorf("Failed %q test: expected result %+v, but got %+v", name, data.Map, result)
		}
	}
}

func TestEnvarsFromMapToSlice(t *testing.T) {
	for name, data := range testData {
		result := EnvarsFromMapToSlice(data.Map)

		if !slices.Equal(data.Slice, result) {
			t.Errorf("Failed %q test: expected result %+v, but got %+v", name, data.Slice, result)
		}
	}
}
