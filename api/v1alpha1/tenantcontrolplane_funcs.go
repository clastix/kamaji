// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AssignedControlPlaneAddress returns the announced address and port of a Tenant Control Plane.
// In case of non-well formed values, or missing announcement, an error is returned.
func (in *TenantControlPlane) AssignedControlPlaneAddress() (string, int32, error) {
	if len(in.Status.ControlPlaneEndpoint) == 0 {
		return "", 0, fmt.Errorf("the Tenant Control Plane is not yet exposed")
	}

	address, portString, err := net.SplitHostPort(in.Status.ControlPlaneEndpoint)
	if err != nil {
		return "", 0, fmt.Errorf("cannot split host port from Tenant Control Plane endpoint: %w", err)
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, fmt.Errorf("cannot convert Tenant Control Plane port from endpoint: %w", err)
	}

	return address, int32(port), nil
}

// DeclaredControlPlaneAddress returns the desired Tenant Control Plane address.
// For services, it returns the clusterIP if available, otherwise the DNS name.
// When the address cannot be determined, an error is returned.
func (in *TenantControlPlane) DeclaredControlPlaneAddress(ctx context.Context, client client.Client) (string, error) {
	switch {
	case len(in.Spec.NetworkProfile.Address) > 0:
		// Returning the hard-coded value in the specification in case of non LoadBalanced resources
		return in.Spec.NetworkProfile.Address, nil
	default:
		// Try to get the service clusterIP, otherwise use DNS name
		svc := &corev1.Service{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: in.GetNamespace(), Name: in.GetName()}, svc); err != nil {
			return "", fmt.Errorf("cannot retrieve Service for the TenantControlPlane: %w", err)
		}
		if len(svc.Spec.ClusterIP) > 0 {
			return svc.Spec.ClusterIP, nil
		}
		// Fall back to DNS name if clusterIP is not yet assigned
		return fmt.Sprintf("%s.%s.svc", in.GetName(), in.GetNamespace()), nil
	}
}

func (in *TenantControlPlane) normalizeNamespaceName() string {
	// The dash character (-) must be replaced with an underscore, PostgreSQL is complaining about it:
	// https://github.com/clastix/kamaji/issues/328
	return strings.ReplaceAll(fmt.Sprintf("%s_%s", in.GetNamespace(), in.GetName()), "-", "_")
}

func (in *TenantControlPlane) GetDefaultDatastoreUsername() string {
	return in.normalizeNamespaceName()
}

func (in *TenantControlPlane) GetDefaultDatastoreSchema() string {
	return in.normalizeNamespaceName()
}
