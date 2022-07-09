// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// KubeadmConfigChecksumDependant is the interface used to retrieve the checksum of the kubeadm phases and addons
// configuration, required to validate the changes and, upon from that, perform the required reconciliation.
// +kubebuilder:object:generate=false
type KubeadmConfigChecksumDependant interface {
	GetChecksum() string
	SetChecksum(string)
}
