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

// gatewayKinds are the Gateway API kinds Kamaji watches. A cluster can serve the group without
// every kind in it, and an informer for an unserved kind never syncs.
var gatewayKinds = []string{"Gateway", "TLSRoute"}

// NOTE: These functions are extremely similar, maybe they can be merged and accept a GVK.
// Explicit for now.
// GatewayAPIResourcesAvailable checks if the Gateway API kinds Kamaji uses are served in the cluster.
func GatewayAPIResourcesAvailable(ctx context.Context, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(gatewayv1.GroupVersion.String())
	if err != nil {
		return false, err
	}

	served := make(map[string]bool, len(resourceList.APIResources))
	for _, resource := range resourceList.APIResources {
		served[resource.Kind] = true
	}

	for _, kind := range gatewayKinds {
		if !served[kind] {
			return false, nil
		}
	}

	return true, nil
}

// IsGatewayAPIGroupAvailable checks if the Gateway API group is served at all, with fallback to client-based check.
func IsGatewayAPIGroupAvailable(ctx context.Context, c client.Client, discoveryClient discovery.DiscoveryInterface) bool {
	if discoveryClient == nil {
		return IsGatewayAPIAvailableViaClient(ctx, c)
	}

	serverGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return false
	}

	for _, group := range serverGroups.Groups {
		if group.Name == gatewayv1.GroupName {
			return true
		}
	}

	return false
}

// TLSRouteAPIAvailable checks specifically for TLSRoute resource availability.
func TLSRouteAPIAvailable(ctx context.Context, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gv := gatewayv1.GroupVersion

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
	return isGatewayKindAvailableViaClient(c, "TLSRoute")
}

// IsGatewayAPIAvailableViaClient uses client to check that the Gateway API kinds Kamaji uses are available.
func IsGatewayAPIAvailableViaClient(ctx context.Context, c client.Client) bool {
	for _, kind := range gatewayKinds {
		if !isGatewayKindAvailableViaClient(c, kind) {
			return false
		}
	}

	return true
}

func isGatewayKindAvailableViaClient(c client.Client, kind string) bool {
	gvk := schema.GroupVersionKind{
		Group:   gatewayv1.GroupName,
		Version: gatewayv1.GroupVersion.Version,
		Kind:    kind,
	}

	_, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false
		}
		// Other errors might be transient, assume available
		return true
	}

	return true
}
