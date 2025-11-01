// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	case tcp.Spec.ControlPlane.GatewayRoute == nil && tcp.Status.Kubernetes.GatewayRoutes == nil:
		return false
	case tcp.Spec.ControlPlane.GatewayRoute != nil && tcp.Status.Kubernetes.GatewayRoutes == nil:
		return true
	case tcp.Spec.ControlPlane.GatewayRoute == nil && tcp.Status.Kubernetes.GatewayRoutes != nil:
		return true
	case tcp.Spec.ControlPlane.GatewayRoute != nil && tcp.Status.Kubernetes.GatewayRoutes != nil:
		// Both spec and status have gateway configuration - check if status needs updating
		// For now, assume it always needs updating to keep status fresh
		return true
	}

	return false
}

func (r *KubernetesGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.GatewayRoute == nil
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
		logger.Info("skipping cleanup: HTTP and gRPC Routes is not managed by Kamaji", "name", route.Name, "namespace", route.Namespace)
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

func (r *KubernetesGatewayResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	// TODO: Rework this.

	logger.V(1).Info("updating TenantControlPlane status for Gateway routes")
	if tenantControlPlane.Spec.ControlPlane.GatewayRoute != nil {
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

	// Clean up status if Gateway routes are no longer configured
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
			tenantControlPlane.Spec.ControlPlane.GatewayRoute.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tenantControlPlane.Spec.ControlPlane.GatewayRoute.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		if tenantControlPlane.Spec.ControlPlane.GatewayRoute.GatewayParentRef != nil {
			r.resource.Spec.ParentRefs = tenantControlPlane.Spec.ControlPlane.GatewayRoute.GatewayParentRef
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
		if len(tenantControlPlane.Spec.ControlPlane.GatewayRoute.Hostname) == 0 {
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

		r.resource.Spec.Hostnames = tenantControlPlane.Spec.ControlPlane.GatewayRoute.Hostname
		r.resource.Spec.Rules = []gatewayv1alpha2.TLSRouteRule{rule}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesGatewayResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if tenantControlPlane.Spec.ControlPlane.GatewayRoute == nil {
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

	// TODO: DEAD CODE
	if result != controllerutil.OperationResultNone {
		return result, nil
	}
	return result, nil
}

func (r *KubernetesGatewayResource) GetName() string {
	return "gateway_routes"
}
