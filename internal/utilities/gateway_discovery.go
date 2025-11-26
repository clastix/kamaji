// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// AreGatewayResourcesAvailable checks if Gateway API is available in the cluster through a discovery Client
// with fallback to client-based check.
func AreGatewayResourcesAvailable(ctx context.Context, c client.Client, discoveryClient discovery.DiscoveryInterface) bool {
	if discoveryClient == nil {
		return IsGatewayAPIAvailableViaClient(ctx, c)
	}

	available, err := GatewayAPIResourcesAvailable(ctx, discoveryClient)
	if err != nil {
		return false
	}

	return available
}

// NOTE: These functions are extremely similar, maybe they can be merged and accept a GVK.
// Explicit for now.
// GatewayAPIResourcesAvailable checks if Gateway API is available in the cluster.
func GatewayAPIResourcesAvailable(ctx context.Context, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gatewayAPIGroup := gatewayv1.GroupName

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

// TLSRouteAPIAvailable checks specifically for TLSRoute resource availability.
func TLSRouteAPIAvailable(ctx context.Context, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gv := gatewayv1alpha2.SchemeGroupVersion

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

// IsTLSRouteAvailable checks if TLSRoute is available with fallback to client-based check.
func IsTLSRouteAvailable(ctx context.Context, c client.Client, discoveryClient discovery.DiscoveryInterface) bool {
	if discoveryClient == nil {
		return IsTLSRouteAvailableViaClient(ctx, c)
	}

	available, err := TLSRouteAPIAvailable(ctx, discoveryClient)
	if err != nil {
		return false
	}

	return available
}

// IsTLSRouteAvailableViaClient uses client to check TLSRoute availability.
func IsTLSRouteAvailableViaClient(ctx context.Context, c client.Client) bool {
	// Try to check if TLSRoute GVK can be resolved
	gvk := schema.GroupVersionKind{
		Group:   gatewayv1alpha2.GroupName,
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

// IsGatewayAPIAvailableViaClient uses client to check Gateway API availability.
func IsGatewayAPIAvailableViaClient(ctx context.Context, c client.Client) bool {
	return IsTLSRouteAvailableViaClient(ctx, c)
}
