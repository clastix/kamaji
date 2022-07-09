// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in AddonStatus) GetChecksum() string {
	return in.Checksum
}

func (in *AddonStatus) SetChecksum(checksum string) {
	in.LastUpdate = metav1.Now()
	in.Checksum = checksum
}
