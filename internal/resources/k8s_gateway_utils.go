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

		// Use the indexer to efficiently find the gateway with the specific listener
		gateway, err := fetchGatewayByListener(ctx, c, routeStatus.ParentRef)
		if err != nil {
			return nil, fmt.Errorf("could not fetch gateway with listener '%v': %w",
				routeStatus.ParentRef.SectionName, err)
		}
		gatewayProgrammed := meta.IsStatusConditionTrue(
			gateway.Status.Conditions,
			string(gatewayv1.GatewayConditionProgrammed),
		)
		if !gatewayProgrammed {
			continue
		}

		// Since we fetched the gateway using the indexer, we know the listener exists
		// but we still need to get its details from the gateway spec
		listener, err := FindMatchingListener(
			gateway.Spec.Listeners, routeStatus.ParentRef,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to match listener: %w", err)
		}

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

	return accessPoints, nil
}

// NewParentRefsSpecWithPortAndSection creates a copy of parentRefs with port and sectionName set for each reference.
func NewParentRefsSpecWithPortAndSection(parentRefs []gatewayv1.ParentReference, port int32, sectionName string) []gatewayv1.ParentReference {
	result := make([]gatewayv1.ParentReference, len(parentRefs))
	sectionNamePtr := gatewayv1.SectionName(sectionName)
	for i, parentRef := range parentRefs {
		result[i] = *parentRef.DeepCopy()
		result[i].Port = &port
		result[i].SectionName = &sectionNamePtr
	}

	return result
}
