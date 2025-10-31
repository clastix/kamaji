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
		// No update in case of no gateway in spec, neither in status.
		return false
	case tcp.Spec.ControlPlane.GatewayRoutes != nil && tcp.Status.Kubernetes.GatewayRoutes == nil: // TCP is using a Gateway, Status not tracking it
		return true
	case tcp.Spec.ControlPlane.GatewayRoutes == nil && tcp.Status.Kubernetes.GatewayRoutes != nil: // Status tracks a Gateway, Spec doesn't
		return true
	default:
		// TODO: IMPLEMENT ME.
	}

	return false
}

func (r *KubernetesGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.GatewayRoutes == nil
}

func (r *KubernetesGatewayResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

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

	return true, nil
}

func (r *KubernetesGatewayResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	// TODO: IMPLEMENT ME.
	// if tenantControlPlane.Spec.ControlPlane.GatewayRoutes != nil {
	// 	tenantControlPlane.Status.Kubernetes.GatewayRoutes = &kamajiv1alpha1.KubernetesGatewayRoutesStatus{
	// 		HTTPRouteStatus: &gatewayv1.HTTPRouteStatus{

	// 		},
	// 	}

	// 	return nil
	// }

	// tenantControlPlane.Status.Kubernetes.Ingress = nil

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

		// Fail if no hostname is specified, same as the ingress resource.
		if len(tenantControlPlane.Spec.ControlPlane.GatewayRoutes.Hostname) == 0 {
			return fmt.Errorf("missing hostname to expose the Tenant Control Plane using a Gateway resource")
		}

		r.httpResource.Spec.Rules = []gatewayv1.HTTPRouteRule{httpRule}

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

		if len(tenantControlPlane.Spec.ControlPlane.GatewayRoutes.Hostname) > 0 {
			r.grpcResource.Spec.Hostnames = tenantControlPlane.Spec.ControlPlane.GatewayRoutes.Hostname
		}

		r.grpcResource.Spec.Rules = []gatewayv1.GRPCRouteRule{grpcRule}

		if err := controllerutil.SetControllerReference(tenantControlPlane, r.httpResource, r.Client.Scheme()); err != nil {
			return err
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.grpcResource, r.Client.Scheme())
	}
}

func (r *KubernetesGatewayResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	// Create or update HTTPRoute
	httpResult, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.httpResource, r.mutate(tenantControlPlane))
	if err != nil {
		return httpResult, err
	}

	// Create or update GRPCRoute
	grpcResult, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.grpcResource, r.mutate(tenantControlPlane))
	if err != nil {
		return grpcResult, err
	}

	// Return the most significant result
	if httpResult != controllerutil.OperationResultNone {
		return httpResult, nil
	}
	return grpcResult, nil
}

func (r *KubernetesGatewayResource) GetName() string {
	return "gateway_routes"
}
