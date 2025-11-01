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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesGatewayResource struct {
	httpResource *gatewayv1.HTTPRoute
	grpcResource *gatewayv1.GRPCRoute
	Client       client.Client
}

func (r *KubernetesGatewayResource) GetHistogram() prometheus.Histogram {
	gatewayCollector = LazyLoadHistogramFromResource(gatewayCollector, r)
	return gatewayCollector
}

func (r *KubernetesGatewayResource) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.ControlPlane.GatewayRoutes == nil && tcp.Status.Kubernetes.GatewayRoutes == nil:
		return false
	case tcp.Spec.ControlPlane.GatewayRoutes != nil && tcp.Status.Kubernetes.GatewayRoutes == nil:
		return true
	case tcp.Spec.ControlPlane.GatewayRoutes == nil && tcp.Status.Kubernetes.GatewayRoutes != nil:
		return true
	case tcp.Spec.ControlPlane.GatewayRoutes != nil && tcp.Status.Kubernetes.GatewayRoutes != nil:
		// Both spec and status have gateway configuration - check if status needs updating
		// For now, assume it always needs updating to keep status fresh
		return true
	}

	return false
}

func (r *KubernetesGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.GatewayRoutes == nil
}

func (r *KubernetesGatewayResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if r.httpResource == nil || r.grpcResource == nil {
		logger.Info("Gateway resources not defined, nothing to clean up")
		return false, nil
	}

	var httpRoute gatewayv1.HTTPRoute
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: r.httpResource.GetNamespace(),
		Name:      r.httpResource.GetName(),
	}, &httpRoute); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to get httpRoute resource before cleanup")

			return false, err
		}

		return false, nil
	}

	var grpcRoute gatewayv1.GRPCRoute
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: r.grpcResource.GetNamespace(),
		Name:      r.grpcResource.GetName(),
	}, &grpcRoute); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to get grpcRoute resource before cleanup")

			return false, err
		}

		return false, nil
	}

	if !metav1.IsControlledBy(&httpRoute, tcp) || !metav1.IsControlledBy(&grpcRoute, tcp) {
		logger.Info("skipping cleanup: HTTP and gRPC Routes is not managed by Kamaji", "name", httpRoute.Name, "namespace", httpRoute.Namespace)

		return false, nil
	}

	if err := r.Client.Delete(ctx, &httpRoute); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot cleanup resource")

			return false, err
		}

		return false, nil
	}

	if err := r.Client.Delete(ctx, &grpcRoute); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot cleanup resource")

			return false, err
		}

		return false, nil
	}

	logger.V(1).Info("gateway routes resource cleaned up successfully")
	return true, nil
}

func (r *KubernetesGatewayResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	logger.V(1).Info("updating TenantControlPlane status for Gateway routes")
	if tenantControlPlane.Spec.ControlPlane.GatewayRoutes != nil {
		// Initialize route statuses with minimal required fields
		httpStatus := gatewayv1.HTTPRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		}
		grpcStatus := gatewayv1.GRPCRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		}

		// If the actual resources exist and have status, use that instead
		if len(r.httpResource.Status.Parents) > 0 {
			httpStatus = r.httpResource.Status
		}
		if len(r.grpcResource.Status.Parents) > 0 {
			grpcStatus = r.grpcResource.Status
		}

		tenantControlPlane.Status.Kubernetes.GatewayRoutes = &kamajiv1alpha1.KubernetesGatewayRoutesStatus{
			Name:            r.httpResource.GetName(),
			Namespace:       r.httpResource.GetNamespace(),
			HTTPRouteStatus: &httpStatus,
			GRPCRouteStatus: &grpcStatus,
		}

		return nil
	}

	// Clean up status if Gateway routes are no longer configured
	tenantControlPlane.Status.Kubernetes.GatewayRoutes = nil

	return nil
}

func (r *KubernetesGatewayResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.httpResource = &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	r.grpcResource = &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesGatewayResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		labels := utilities.MergeMaps(r.httpResource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()), tenantControlPlane.Spec.ControlPlane.GatewayRoutes.AdditionalMetadata.Labels)
		r.httpResource.SetLabels(labels)
		r.grpcResource.SetLabels(labels)

		annotations := utilities.MergeMaps(r.httpResource.GetAnnotations(), tenantControlPlane.Spec.ControlPlane.GatewayRoutes.AdditionalMetadata.Annotations)
		r.httpResource.SetAnnotations(annotations)
		r.grpcResource.SetAnnotations(annotations)

		if tenantControlPlane.Spec.ControlPlane.GatewayRoutes.GatewayParentRefs != nil {
			r.httpResource.Spec.ParentRefs = tenantControlPlane.Spec.ControlPlane.GatewayRoutes.GatewayParentRefs
			r.grpcResource.Spec.ParentRefs = tenantControlPlane.Spec.ControlPlane.GatewayRoutes.GatewayParentRefs
		}

		if tenantControlPlane.Status.Kubernetes.Service.Name == "" ||
			tenantControlPlane.Status.Kubernetes.Service.Port == 0 {
			return fmt.Errorf("gateway cannot be configured yet, service not ready")
		}

		serviceName := gatewayv1.ObjectName(tenantControlPlane.Status.Kubernetes.Service.Name)
		servicePort := gatewayv1.PortNumber(tenantControlPlane.Status.Kubernetes.Service.Port)

		// Fail if no hostname is specified, same as the ingress resource.
		if len(tenantControlPlane.Spec.ControlPlane.GatewayRoutes.Hostnames) == 0 {
			return fmt.Errorf("missing hostname to expose the Tenant Control Plane using a Gateway resource")
		}

		httpRule := gatewayv1.HTTPRouteRule{
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: serviceName,
							Port: &servicePort,
						},
					},
				},
			},
		}

		grpcRule := gatewayv1.GRPCRouteRule{
			BackendRefs: []gatewayv1.GRPCBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: serviceName,
							Port: &servicePort,
						},
					},
				},
			},
		}

		r.httpResource.Spec.Hostnames = tenantControlPlane.Spec.ControlPlane.GatewayRoutes.Hostnames
		r.httpResource.Spec.Rules = []gatewayv1.HTTPRouteRule{httpRule}

		r.grpcResource.Spec.Hostnames = tenantControlPlane.Spec.ControlPlane.GatewayRoutes.Hostnames
		r.grpcResource.Spec.Rules = []gatewayv1.GRPCRouteRule{grpcRule}

		if err := controllerutil.SetControllerReference(tenantControlPlane, r.httpResource, r.Client.Scheme()); err != nil {
			return err
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.grpcResource, r.Client.Scheme())
	}
}

func (r *KubernetesGatewayResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if tenantControlPlane.Spec.ControlPlane.GatewayRoutes == nil {
		return controllerutil.OperationResultNone, nil
	}

	logger.V(1).Info("creating or updating resource gateway routes")
	
	// Create fresh resources to avoid resourceVersion conflicts
	httpResource := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}
	
	grpcResource := &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}
	
	// Store the fresh resources for status updates
	r.httpResource = httpResource
	r.grpcResource = grpcResource
	
	httpResult, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, httpResource, r.mutate(tenantControlPlane))
	if err != nil {
		return httpResult, err
	}

	grpcResult, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, grpcResource, r.mutate(tenantControlPlane))
	if err != nil {
		return grpcResult, err
	}

	if httpResult != controllerutil.OperationResultNone {
		return httpResult, nil
	}
	return grpcResult, nil
}

func (r *KubernetesGatewayResource) GetName() string {
	return "gateway_routes"
}
