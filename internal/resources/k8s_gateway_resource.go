// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	case tcp.Spec.ControlPlane.Gateway == nil && tcp.Status.Kubernetes.GatewayRoutes == nil:
		return false
	case tcp.Spec.ControlPlane.Gateway != nil && tcp.Status.Kubernetes.GatewayRoutes == nil:
		return true
	case tcp.Spec.ControlPlane.Gateway == nil && tcp.Status.Kubernetes.GatewayRoutes != nil:
		return true
	case tcp.Spec.ControlPlane.Gateway != nil && tcp.Status.Kubernetes.GatewayRoutes != nil:
		// Both spec and status have gateway configuration - check if status needs updating
		// For now, assume it always needs updating to keep status fresh
		return true
	}

	return false
}

func (r *KubernetesGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.Gateway == nil
}

func (r *KubernetesGatewayResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if r.resource == nil {
		logger.Info("TLSRoute is not defined, nothing to clean up")
		return false, nil
	}

	var route = gatewayv1alpha2.TLSRoute{}
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

func (r *KubernetesGatewayResource) fetchGateway(ctx context.Context, ref gatewayv1.ParentReference) (*gatewayv1.Gateway, error) {
	if ref.Namespace == nil {
		return nil, fmt.Errorf("missing namespace")
	}

	gateway := &gatewayv1.Gateway{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      string(ref.Name),
		Namespace: string(*ref.Namespace),
	}, gateway)

	return gateway, err
}

func findMatchingListener(
	gateway *gatewayv1.Gateway,
	ref gatewayv1.ParentReference,
) (gatewayv1.Listener, error) {
	if ref.SectionName == nil {
		return gatewayv1.Listener{}, fmt.Errorf("missing sectionName")
	}
	name := *ref.SectionName
	for _, listener := range gateway.Spec.Listeners {
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

func extractAddresses(status gatewayv1.GatewayStatus) (addresses []string, err error) {
	for _, addr := range status.Addresses {
		if addr.Type == nil || *addr.Type != gatewayv1.IPAddressType {
			return nil, fmt.Errorf("unknown type: %v", addr.Type)
		}
		addresses = append(addresses, addr.Value)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no address")
	}
	return
}

func (r *KubernetesGatewayResource) computeEndpoints(
	ctx context.Context,
) (ipURLs []*url.URL, fqdnURLs []*url.URL, err error) {

	if len(r.resource.Status.Parents) == 0 {
		return nil, nil, fmt.Errorf("route has no gateway")
	}

	// TODO: Make singular.
	if len(r.resource.Status.Parents) > 1 {
		return nil, nil, fmt.Errorf("route has more than one gateway")
	}

	ref := r.resource.Status.Parents[0].ParentRef
	gw, err := r.fetchGateway(ctx, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	listener, err := findMatchingListener(gw, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to match listener: %w", err)
	}

	addresses, err := extractAddresses(gw.Status)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract addresses: %w", err)
	}

	for _, addr := range addresses {
		rawURL := fmt.Sprint("https://%s:%i", addr, listener.Port)
		url, err := url.Parse(rawURL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid url: %w", err)
		}
		fqdnURLs = append(fqdnURLs, url)
	}
	for _, hostname := range r.resource.Spec.Hostnames {
		rawURL := fmt.Sprint("https://%s:%i", hostname, listener.Port)
		url, err := url.Parse(rawURL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid url: %w", err)
		}
		ipURLs = append(ipURLs, url)
	}
	return
}

func (r *KubernetesGatewayResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	// Clean up status if Gateway routes are no longer configured
	if tenantControlPlane.Spec.ControlPlane.Gateway == nil {
		tenantControlPlane.Status.Kubernetes.GatewayRoutes = nil
		return nil
	}

	// TODO: check the conditions of the route.

	ipEndpoints, fqdnEndpoints, err := r.computeEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("could not compute endpoints: %w", err)
	}

	// TODO: Ultimately, given a TLSRoute, we should be able to create a URL.
	//

	logger.V(1).Info("updating TenantControlPlane status for Gateway routes")
	if tenantControlPlane.Spec.ControlPlane.Gateway != nil {
		// TODO: Evaluate the conditions and report a better status.
		routeStatus := gatewayv1alpha2.TLSRouteStatus{
			RouteStatus: gatewayv1alpha2.RouteStatus{
				Parents: []gatewayv1alpha2.RouteParentStatus{},
			},
		}

		// If the actual resources exist and have status, use that instead
		if len(r.resource.Status.Parents) > 0 {
			routeStatus = r.resource.Status
		}

		tenantControlPlane.Status.Kubernetes.GatewayRoutes = &kamajiv1alpha1.KubernetesGatewayRoutesStatus{
			Name:           r.resource.GetName(),
			Namespace:      r.resource.GetNamespace(),
			TLSRouteStatus: &routeStatus,
		}

		return nil
	}

	tenantControlPlane.Status.Kubernetes.GatewayRoutes = nil

	return nil
}

func (r *KubernetesGatewayResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesGatewayResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()),
			tenantControlPlane.Spec.ControlPlane.Gateway.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tenantControlPlane.Spec.ControlPlane.Gateway.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		if tenantControlPlane.Spec.ControlPlane.Gateway.GatewayParentRefs != nil {
			r.resource.Spec.ParentRefs = tenantControlPlane.Spec.ControlPlane.Gateway.GatewayParentRefs
		}

		// TODO: Make sure that we are listening on this?
		if tenantControlPlane.Status.Kubernetes.Service.Name == "" ||
			tenantControlPlane.Status.Kubernetes.Service.Port == 0 {
			// TODO: Is error correct here, we should just retry.
			return fmt.Errorf("gateway cannot be configured yet, service not ready")
		}

		serviceName := gatewayv1alpha2.ObjectName(tenantControlPlane.Status.Kubernetes.Service.Name)
		servicePort := gatewayv1alpha2.PortNumber(tenantControlPlane.Status.Kubernetes.Service.Port)

		// Fail if no hostname is specified, same as the ingress resource.
		if len(tenantControlPlane.Spec.ControlPlane.Gateway.Hostnames) == 0 {
			return fmt.Errorf("missing hostname to expose the Tenant Control Plane using a Gateway resource")
		}

		rule := gatewayv1alpha2.TLSRouteRule{
			BackendRefs: []gatewayv1alpha2.BackendRef{
				{
					BackendObjectReference: gatewayv1alpha2.BackendObjectReference{
						Name: serviceName,
						// TODO: Why a pointer here?
						Port: &servicePort,
					},
				},
			},
		}

		r.resource.Spec.Hostnames = tenantControlPlane.Spec.ControlPlane.Gateway.Hostnames
		r.resource.Spec.Rules = []gatewayv1alpha2.TLSRouteRule{rule}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesGatewayResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if tenantControlPlane.Spec.ControlPlane.Gateway == nil {
		return controllerutil.OperationResultNone, nil
	}

	logger.V(1).Info("creating or updating resource gateway routes")

	// Create fresh resources to avoid resourceVersion conflicts
	route := &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	// Store the fresh resources for status updates
	r.resource = route

	result, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, route, r.mutate(tenantControlPlane))
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *KubernetesGatewayResource) GetName() string {
	return "gateway_routes"
}
