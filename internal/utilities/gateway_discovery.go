// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: These functions are extremely similar, maybe they can be merged and accept a GVK.
// Explicit for now.
// IsGatewayAPIAvailable checks if Gateway API is available in the cluster
func IsGatewayAPIAvailable(ctx context.Context, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gatewayAPIGroup := "gateway.networking.k8s.io"

	serverGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, err
	}

	for _, group := range serverGroups.Groups {
		if group.Name == gatewayAPIGroup {
			return true, nil
		}
	}

	return false, nil
}

// IsTLSRouteAPIAvailable checks specifically for TLSRoute resource availability
func IsTLSRouteAPIAvailable(ctx context.Context, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gv := schema.GroupVersion{Group: "gateway.networking.k8s.io", Version: "v1alpha2"}

	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return false, err
	}

	for _, resource := range resourceList.APIResources {
		if resource.Kind == "TLSRoute" {
			return true, nil
		}
	}

	return false, nil
}

// IsGatewayAPIAvailableViaClient uses client to check Gateway API availability
func IsGatewayAPIAvailableViaClient(ctx context.Context, c client.Client) bool {
	// Try to check if TLSRoute GVK can be resolved
	gvk := schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1alpha2",
		Kind:    "TLSRoute",
	}

	restMapper := c.RESTMapper()
	_, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false
		}
		// Other errors might be transient, assume available
		return true
	}

	return true
}
