// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"net/url"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// fetchGatewayByListener uses the indexer to efficiently find a gateway with a specific listener.
// This avoids the need to iterate through all listeners in a gateway.
func fetchGatewayByListener(ctx context.Context, c client.Client, ref gatewayv1.ParentReference) (*gatewayv1.Gateway, error) {
	if ref.SectionName == nil {
		return nil, fmt.Errorf("missing sectionName")
	}

	// Build the composite key that matches our indexer format: namespace/gatewayName/listenerName
	listenerKey := fmt.Sprintf("%s/%s/%s", *ref.Namespace, ref.Name, *ref.SectionName)

	// Query gateways using the indexer
	gatewayList := &gatewayv1.GatewayList{}
	if err := c.List(ctx, gatewayList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(kamajiv1alpha1.GatewayListenerNameKey, listenerKey),
	}); err != nil {
		return nil, fmt.Errorf("failed to list gateways by listener: %w", err)
	}

	if len(gatewayList.Items) == 0 {
		return nil, fmt.Errorf("no gateway found with listener '%s'", *ref.SectionName)
	}

	// Since we're using a composite key with namespace/name/listener, we should get exactly one result
	if len(gatewayList.Items) > 1 {
		return nil, fmt.Errorf("found multiple gateways with listener '%s', expected exactly one", *ref.SectionName)
	}

	return &gatewayList.Items[0], nil
}

// FindMatchingListener finds a listener in the given list that matches the parent reference.
func FindMatchingListener(listeners []gatewayv1.Listener, ref gatewayv1.ParentReference) (gatewayv1.Listener, error) {
	if ref.SectionName == nil {
		return gatewayv1.Listener{}, fmt.Errorf("missing sectionName")
	}
	name := *ref.SectionName
	for _, listener := range listeners {
		if listener.Name == name {
			return listener, nil
		}
	}

	// TODO: Handle the cases according to the spec:
	//  - When both Port (experimental) and SectionName are
	//    specified, the name and port of the selected listener
	//    must match both specified values.
	//  - When unspecified (empty string) this will reference
	//    the entire resource [...] an attachment is considered
	//     successful if at least one section in the parent resource accepts it

	return gatewayv1.Listener{}, fmt.Errorf("could not find listener '%s'", name)
}

// IsGatewayRouteStatusChanged checks if the gateway route status has changed compared to the stored status.
// Returns true if the status has changed (update needed), false if it's the same.
func IsGatewayRouteStatusChanged(currentStatus *kamajiv1alpha1.KubernetesGatewayStatus, resourceStatus gatewayv1alpha2.RouteStatus) bool {
	if currentStatus == nil {
		return true
	}

	// Compare RouteStatus - check if number of parents changed
	if len(currentStatus.RouteStatus.Parents) != len(resourceStatus.Parents) {
		return true
	}

	// Compare individual parent statuses
	// NOTE: Multiple Parent References are assumed.
	for i, currentParent := range currentStatus.RouteStatus.Parents {
		if i >= len(resourceStatus.Parents) {
			return true
		}

		resourceParent := resourceStatus.Parents[i]

		// Compare parent references
		if currentParent.ParentRef.Name != resourceParent.ParentRef.Name ||
			(currentParent.ParentRef.Namespace == nil) != (resourceParent.ParentRef.Namespace == nil) ||
			(currentParent.ParentRef.Namespace != nil && resourceParent.ParentRef.Namespace != nil &&
				*currentParent.ParentRef.Namespace != *resourceParent.ParentRef.Namespace) ||
			(currentParent.ParentRef.SectionName == nil) != (resourceParent.ParentRef.SectionName == nil) ||
			(currentParent.ParentRef.SectionName != nil && resourceParent.ParentRef.SectionName != nil &&
				*currentParent.ParentRef.SectionName != *resourceParent.ParentRef.SectionName) {
			return true
		}

		if len(currentParent.Conditions) != len(resourceParent.Conditions) {
			return true
		}

		// Compare each condition
		for j, currentCondition := range currentParent.Conditions {
			if j >= len(resourceParent.Conditions) {
				return true
			}

			resourceCondition := resourceParent.Conditions[j]

			if currentCondition.Type != resourceCondition.Type ||
				currentCondition.Status != resourceCondition.Status ||
				currentCondition.Reason != resourceCondition.Reason ||
				currentCondition.Message != resourceCondition.Message ||
				!currentCondition.LastTransitionTime.Equal(&resourceCondition.LastTransitionTime) {
				return true
			}
		}
	}

	// Since access points are derived from route status and gateway conditions,
	// and we've already compared the route status above, we can assume that
	// if the route status hasn't changed, the access points calculation
	// will produce the same result. This avoids the need for complex
	// gateway fetching in the status comparison.
	//
	// If there are edge cases where gateway state changes but route status doesn't,
	// those will be caught in the next reconciliation cycle anyway.
	return false
}

// CleanupTLSRoute cleans up a TLSRoute resource if it's managed by the given TenantControlPlane.
func CleanupTLSRoute(ctx context.Context, c client.Client, routeName, routeNamespace string, tcp metav1.Object) (bool, error) {
	route := gatewayv1alpha2.TLSRoute{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: routeNamespace,
		Name:      routeName,
	}, &route); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, fmt.Errorf("failed to get TLSRoute before cleanup: %w", err)
		}

		return false, nil
	}

	if !metav1.IsControlledBy(&route, tcp) {
		return false, nil
	}

	if err := c.Delete(ctx, &route); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, fmt.Errorf("cannot delete TLSRoute route: %w", err)
		}

		return false, nil
	}

	return true, nil
}

// BuildGatewayAccessPointsStatus builds access points from route statuses.
//
// Per the Gateway API specification, ParentReference.SectionName is optional:
// when unset (or empty), the Route attaches to every listener of the referenced
// Gateway that accepts it (typically via port/protocol matching). We support
// both cases:
//
//   - SectionName is set: resolve the Gateway via the listener-name indexer and
//     build a single access point for that listener.
//   - SectionName is nil/empty: resolve the Gateway by namespace/name and build
//     an access point for each listener, optionally filtered by ParentRef.Port.
//
// Unresolvable or unprogrammed Gateways are skipped rather than returned as
// errors, so that a single mis-attached parentRef does not block the whole
// status update for the Route.
func BuildGatewayAccessPointsStatus(ctx context.Context, c client.Client, route *gatewayv1alpha2.TLSRoute, routeStatuses gatewayv1alpha2.RouteStatus) ([]kamajiv1alpha1.GatewayAccessPoint, error) {
	accessPoints := []kamajiv1alpha1.GatewayAccessPoint{}
	routeNamespace := gatewayv1.Namespace(route.Namespace)

	for _, routeStatus := range routeStatuses.Parents {
		routeAccepted := meta.IsStatusConditionTrue(
			routeStatus.Conditions,
			string(gatewayv1.RouteConditionAccepted),
		)
		if !routeAccepted {
			continue
		}

		if routeStatus.ParentRef.Namespace == nil {
			// Set the namespace to the route namespace if not set
			routeStatus.ParentRef.Namespace = &routeNamespace
		}

		listeners, err := resolveMatchingListeners(ctx, c, routeStatus.ParentRef)
		if err != nil {
			return nil, fmt.Errorf("could not resolve gateway listeners for parentRef '%s/%s': %w",
				*routeStatus.ParentRef.Namespace, routeStatus.ParentRef.Name, err)
		}

		for _, listener := range listeners {
			for _, hostname := range route.Spec.Hostnames {
				rawURL := fmt.Sprintf("https://%s:%d", hostname, listener.Port)
				parsedURL, err := url.Parse(rawURL)
				if err != nil {
					return nil, fmt.Errorf("invalid url: %w", err)
				}

				hostnameAddressType := gatewayv1.HostnameAddressType
				accessPoints = append(accessPoints, kamajiv1alpha1.GatewayAccessPoint{
					Type:  &hostnameAddressType,
					Value: parsedURL.String(),
					Port:  listener.Port,
				})
			}
		}
	}

	return accessPoints, nil
}

// resolveMatchingListeners returns the listeners of the Gateway referenced by
// ref that should contribute access points for the enclosing Route.
//
// When ref.SectionName is set, exactly one listener is returned (looked up via
// the name indexer for efficiency). When SectionName is nil or empty, every
// listener of the Gateway is considered and, if ref.Port is set, further
// filtered to listeners exposing that port.
//
// If the referenced Gateway is not found, is not Programmed, or has no
// matching listener, an empty slice is returned with a nil error: a single
// unresolvable parentRef must not fail the whole status update.
func resolveMatchingListeners(ctx context.Context, c client.Client, ref gatewayv1.ParentReference) ([]gatewayv1.Listener, error) {
	hasSectionName := ref.SectionName != nil && *ref.SectionName != ""

	// Fast path: look the Gateway up by its specific listener name.
	if hasSectionName {
		gateway, err := fetchGatewayByListener(ctx, c, ref)
		if err != nil {
			// No gateway with that listener: skip this parentRef silently.
			return nil, nil //nolint:nilerr
		}

		if !isGatewayProgrammed(gateway) {
			return nil, nil
		}

		listener, err := FindMatchingListener(gateway.Spec.Listeners, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to match listener: %w", err)
		}

		return []gatewayv1.Listener{listener}, nil
	}

	// SectionName unset: resolve the Gateway by namespace/name.
	gateway := &gatewayv1.Gateway{}
	key := k8stypes.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}
	if err := c.Get(ctx, key, gateway); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to fetch gateway %s: %w", key, err)
	}

	if !isGatewayProgrammed(gateway) {
		return nil, nil
	}

	matching := make([]gatewayv1.Listener, 0, len(gateway.Spec.Listeners))
	for _, listener := range gateway.Spec.Listeners {
		if ref.Port != nil && listener.Port != *ref.Port {
			continue
		}

		matching = append(matching, listener)
	}

	return matching, nil
}

func isGatewayProgrammed(gateway *gatewayv1.Gateway) bool {
	return meta.IsStatusConditionTrue(
		gateway.Status.Conditions,
		string(gatewayv1.GatewayConditionProgrammed),
	)
}
