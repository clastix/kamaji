// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import corev1 "k8s.io/api/core/v1"

// HasNamedVolume finds the Volume in the provided slice by its name, returning a boolean if found, and its index.
func HasNamedVolume(volumes []corev1.Volume, name string) (found bool, index int) {
	for i, volume := range volumes {
		if volume.Name == name {
			return true, i
		}
	}

	return false, 0
}

// HasNamedVolumeMount finds the VolumeMount in the provided slice by its name, returning a boolean if found, and its index.
func HasNamedVolumeMount(volumeMounts []corev1.VolumeMount, name string) (found bool, index int) {
	for i, volume := range volumeMounts {
		if volume.Name == name {
			return true, i
		}
	}

	return false, 0
}
