// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if currentStatus != nil && currentStatus.RouteRef.Name != r.resource.Name {
		return true
	}

	// Compare RouteStatus - check if number of parents changed
	return IsGatewayRouteStatusChanged(currentStatus, r.resource.Status.RouteStatus)
}

func (r *KubernetesGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.Gateway == nil && tcp.Status.Kubernetes.Gateway != nil
}

func (r *KubernetesGatewayResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	cleaned, err := CleanupTLSRoute(ctx, r.Client, r.resource.GetName(), r.resource.GetNamespace(), tcp)
	if err != nil {
		logger.Error(err, "failed to cleanup tcp route")

		return false, err
	}

	if cleaned {
		logger.V(1).Info("tcp route cleaned up successfully")
	}

	return cleaned, nil
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
	accessPoints, err := BuildGatewayAccessPointsStatus(ctx, r.Client, r.resource, routeStatuses)
	if err != nil {
		return err
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

		serviceName := gatewayv1alpha2.ObjectName(tcp.Status.Kubernetes.Service.Name)
		servicePort := tcp.Status.Kubernetes.Service.Port

		if serviceName == "" || servicePort == 0 {
			return fmt.Errorf("service not ready, cannot create TLSRoute")
		}

		if tcp.Spec.ControlPlane.Gateway.GatewayParentRefs != nil {
			// Copy parentRefs and explicitly set port and sectionName fields
			r.resource.Spec.ParentRefs = NewParentRefsSpecWithPortAndSection(tcp.Spec.ControlPlane.Gateway.GatewayParentRefs, servicePort, "kube-apiserver")
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
