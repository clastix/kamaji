// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"slices"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayAPIKinds are the gateway.networking.k8s.io/v1 kinds the TenantControlPlane controller watches.
var GatewayAPIKinds = []string{"Gateway", "HTTPRoute", "GRPCRoute", "TLSRoute"}

// MissingGatewayAPIKinds returns the GatewayAPIKinds the API server does not serve.
func MissingGatewayAPIKinds(discoveryClient discovery.DiscoveryInterface) ([]string, error) {
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(gatewayv1.GroupVersion.String())
	if k8serrors.IsNotFound(err) {
		return GatewayAPIKinds, nil
	}

	if err != nil {
		return nil, err
	}

	var missing []string

	for _, kind := range GatewayAPIKinds {
		if !slices.ContainsFunc(resourceList.APIResources, func(r metav1.APIResource) bool { return r.Kind == kind }) {
			missing = append(missing, kind)
		}
	}

	return missing, nil
}
