// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// EnvarsFromSliceToMap transforms a slice of envvar into a map, simplifying the subsequent mangling.
func EnvarsFromSliceToMap(envs []corev1.EnvVar) (m map[string]corev1.EnvVar) {
	m = make(map[string]corev1.EnvVar)

	for _, env := range envs {
		m[env.Name] = env
	}

	return m
}

// EnvarsFromMapToSlice create the slice of env vars, and sorting the resulting output in order to make it idempotent.
func EnvarsFromMapToSlice(envs map[string]corev1.EnvVar) (slice []corev1.EnvVar) {
	for _, env := range envs {
		slice = append(slice, env)
	}

	sort.Slice(slice, func(i, j int) bool {
		return slice[i].Name < slice[j].Name
	})

	return slice
}
