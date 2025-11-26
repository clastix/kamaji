// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesGatewayResource struct {
	resource *gatewayv1alpha2.TLSRoute
	Client   client.Client
}

func (r *KubernetesGatewayResource) GetHistogram() prometheus.Histogram {
	gatewayCollector = LazyLoadHistogramFromResource(gatewayCollector, r)

	return gatewayCollector
}

func (r *KubernetesGatewayResource) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.ControlPlane.Gateway == nil && tcp.Status.Kubernetes.Gateway == nil:
		return false
	case tcp.Spec.ControlPlane.Gateway != nil && tcp.Status.Kubernetes.Gateway == nil:
		return true
	case tcp.Spec.ControlPlane.Gateway == nil && tcp.Status.Kubernetes.Gateway != nil:
		return true
	// Could be an alias for default here since the other cases are covered.
	case tcp.Spec.ControlPlane.Gateway != nil && tcp.Status.Kubernetes.Gateway != nil:
		return r.gatewayStatusNeedsUpdate(tcp)
	}

	return false
}

// gatewayStatusNeedsUpdate compares the current gateway resource status with the stored status.
func (r *KubernetesGatewayResource) gatewayStatusNeedsUpdate(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	currentStatus := tcp.Status.Kubernetes.Gateway

	// Check if route reference has changed
	if currentStatus.RouteRef.Name != r.resource.Name {
		return true
	}

	// Compare RouteStatus - check if number of parents changed
	if len(currentStatus.RouteStatus.Parents) != len(r.resource.Status.RouteStatus.Parents) {
		return true
	}

	// Compare individual parent statuses
	// NOTE: Multiple Parent References are assumed.
	for i, currentParent := range currentStatus.RouteStatus.Parents {
		if i >= len(r.resource.Status.RouteStatus.Parents) {
			return true
		}

		resourceParent := r.resource.Status.RouteStatus.Parents[i]

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

func (r *KubernetesGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.Gateway == nil && tcp.Status.Kubernetes.Gateway != nil
}

func (r *KubernetesGatewayResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	route := gatewayv1alpha2.TLSRoute{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: r.resource.GetNamespace(),
		Name:      r.resource.GetName(),
	}, &route); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to get TLSRoute before cleanup")

			return false, err
		}

		return false, nil
	}

	if !metav1.IsControlledBy(&route, tcp) {
		logger.Info("skipping cleanup: route is not managed by Kamaji", "name", route.Name, "namespace", route.Namespace)

		return false, nil
	}

	if err := r.Client.Delete(ctx, &route); err != nil {
		if !k8serrors.IsNotFound(err) {
			// TODO: Is that an error? Wanted to delete the resource anyways.
			logger.Error(err, "cannot cleanup tcp route")

			return false, err
		}

		return false, nil
	}

	logger.V(1).Info("tcp route cleaned up successfully")

	return true, nil
}

// fetchGatewayByListener uses the indexer to efficiently find a gateway with a specific listener.
// This avoids the need to iterate through all listeners in a gateway.
func (r *KubernetesGatewayResource) fetchGatewayByListener(ctx context.Context, ref gatewayv1.ParentReference) (*gatewayv1.Gateway, error) {
	if ref.Namespace == nil {
		return nil, fmt.Errorf("missing namespace")
	}
	if ref.SectionName == nil {
		return nil, fmt.Errorf("missing sectionName")
	}

	// Build the composite key that matches our indexer format: namespace/gatewayName/listenerName
	listenerKey := fmt.Sprintf("%s/%s/%s", *ref.Namespace, ref.Name, *ref.SectionName)

	// Query gateways using the indexer
	gatewayList := &gatewayv1.GatewayList{}
	if err := r.Client.List(ctx, gatewayList, client.MatchingFieldsSelector{
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

func (r *KubernetesGatewayResource) UpdateTenantControlPlaneStatus(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	// Clean up status if Gateway routes are no longer configured
	if tcp.Spec.ControlPlane.Gateway == nil {
		tcp.Status.Kubernetes.Gateway = nil

		return nil
	}

	tcp.Status.Kubernetes.Gateway = &kamajiv1alpha1.KubernetesGatewayStatus{
		RouteStatus: r.resource.Status.RouteStatus,
		RouteRef: v1.LocalObjectReference{
			Name: r.resource.Name,
		},
	}

	routeStatuses := tcp.Status.Kubernetes.Gateway.RouteStatus

	// TODO: Investigate the implications of having multiple parents / hostnames
	// TODO: Use condition to report?
	if len(routeStatuses.Parents) == 0 {
		return fmt.Errorf("no gateway attached to the route")
	}
	if len(routeStatuses.Parents) > 1 {
		return fmt.Errorf("too many gateway attached to the route")
	}
	if len(r.resource.Spec.Hostnames) == 0 {
		return fmt.Errorf("no hostname in the route")
	}
	if len(r.resource.Spec.Hostnames) > 1 {
		return fmt.Errorf("too many hostnames in the route")
	}

	logger.V(1).Info("updating TenantControlPlane status for Gateway routes")
	accessPoints := []kamajiv1alpha1.GatewayAccessPoint{}
	for _, routeStatus := range routeStatuses.Parents {
		routeAccepted := meta.IsStatusConditionTrue(
			routeStatus.Conditions,
			string(gatewayv1.RouteConditionAccepted),
		)
		if !routeAccepted {
			continue
		}

		// Use the indexer to efficiently find the gateway with the specific listener
		gateway, err := r.fetchGatewayByListener(ctx, routeStatus.ParentRef)
		if err != nil {
			return fmt.Errorf("could not fetch gateway with listener '%v': %w",
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
			return fmt.Errorf("failed to match listener: %w", err)
		}

		for _, hostname := range r.resource.Spec.Hostnames {
			rawURL := fmt.Sprintf("https://%s:%d", hostname, listener.Port)
			url, err := url.Parse(rawURL)
			if err != nil {
				return fmt.Errorf("invalid url: %w", err)
			}

			hostnameAddressType := gatewayv1.HostnameAddressType
			accessPoints = append(accessPoints, kamajiv1alpha1.GatewayAccessPoint{
				Type:  &hostnameAddressType,
				Value: url.String(),
				Port:  listener.Port,
			})
		}
	}
	tcp.Status.Kubernetes.Gateway.AccessPoints = accessPoints

	return nil
}

func (r *KubernetesGatewayResource) Define(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcp.GetName(),
			Namespace: tcp.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesGatewayResource) mutate(tcp *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(tcp.GetName(), r.GetName()),
			tcp.Spec.ControlPlane.Gateway.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tcp.Spec.ControlPlane.Gateway.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		if tcp.Spec.ControlPlane.Gateway.GatewayParentRefs != nil {
			r.resource.Spec.ParentRefs = tcp.Spec.ControlPlane.Gateway.GatewayParentRefs
		}

		serviceName := gatewayv1alpha2.ObjectName(tcp.Status.Kubernetes.Service.Name)
		servicePort := tcp.Status.Kubernetes.Service.Port

		if serviceName == "" || servicePort == 0 {
			return fmt.Errorf("service not ready, cannot create TLSRoute")
		}

		rule := gatewayv1alpha2.TLSRouteRule{
			BackendRefs: []gatewayv1alpha2.BackendRef{
				{
					BackendObjectReference: gatewayv1alpha2.BackendObjectReference{
						Name: serviceName,
						Port: &servicePort,
					},
				},
			},
		}

		r.resource.Spec.Hostnames = []gatewayv1.Hostname{tcp.Spec.ControlPlane.Gateway.Hostname}
		r.resource.Spec.Rules = []gatewayv1alpha2.TLSRouteRule{rule}

		return controllerutil.SetControllerReference(tcp, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesGatewayResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if tenantControlPlane.Spec.ControlPlane.Gateway == nil {
		return controllerutil.OperationResultNone, nil
	}

	if len(tenantControlPlane.Spec.ControlPlane.Gateway.Hostname) == 0 {
		return controllerutil.OperationResultNone, fmt.Errorf("missing hostname to expose the Tenant Control Plane using a Gateway resource")
	}

	logger.V(1).Info("creating or updating resource gateway routes")

	result, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tenantControlPlane))
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *KubernetesGatewayResource) GetName() string {
	return "gateway_routes"
}
