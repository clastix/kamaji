// Copyright 2025 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "cmp"

// PublicControlPlaneAddress returns the public address for the cluster-info ConfigMap.
// If PublicAPIServerAddress is specified, it returns that value; otherwise, it falls back
// to the assigned control plane address. This allows using DNS names in cluster-info
// instead of LoadBalancer IPs. The port is always taken from NetworkProfile.Port.
func (in *TenantControlPlane) PublicControlPlaneAddress() (string, int32, error) {
	port := cmp.Or(in.Spec.NetworkProfile.Port, 6443)

	// If PublicAPIServerAddress is specified, use it for cluster-info
	if publicAddress := in.Spec.ControlPlane.Service.PublicAPIServerAddress; len(publicAddress) > 0 {
		return publicAddress, port, nil
	}

	// Fall back to the assigned control plane address, but use configured port
	assignedAddress, _, err := in.AssignedControlPlaneAddress()
	if err != nil {
		return "", 0, err
	}

	return assignedAddress, port, nil
}