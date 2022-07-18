// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import corev1 "k8s.io/api/core/v1"

// HasNamedContainer finds the Container in the provided slice by its name, returning a boolean if found, and its index.
func HasNamedContainer(container []corev1.Container, name string) (found bool, index int) {
	for i, volume := range container {
		if volume.Name == name {
			return true, i
		}
	}

	return false, 0
}
