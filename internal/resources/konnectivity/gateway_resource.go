// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

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
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesKonnectivityGatewayResource struct {
	resource *gatewayv1alpha2.TLSRoute
	Client   client.Client
}

func (r *KubernetesKonnectivityGatewayResource) GetHistogram() prometheus.Histogram {
	gatewayCollector = resources.LazyLoadHistogramFromResource(gatewayCollector, r)

	return gatewayCollector
}

func (r *KubernetesKonnectivityGatewayResource) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	switch {
	case !r.shouldHaveGateway(tcp) && (tcp.Status.Addons.Konnectivity.Gateway == nil):
		return false
	case r.shouldHaveGateway(tcp) && (tcp.Status.Addons.Konnectivity.Gateway == nil):
		return true
	case !r.shouldHaveGateway(tcp) && (tcp.Status.Addons.Konnectivity.Gateway != nil):
		return true
	case r.shouldHaveGateway(tcp) && (tcp.Status.Addons.Konnectivity.Gateway != nil):
		return r.gatewayStatusNeedsUpdate(tcp)
	}

	return false
}

// shouldHaveGateway checks if Konnectivity gateway should be configured.
// Create when Konnectivity addon is enabled and control plane gateway is configured.
func (r *KubernetesKonnectivityGatewayResource) shouldHaveGateway(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	if tcp.Spec.Addons.Konnectivity == nil { // konnectivity addon is disabled
		return false
	}
	// Create when control plane gateway is configured
	return tcp.Spec.ControlPlane.Gateway != nil
}

// gatewayStatusNeedsUpdate compares the current gateway resource status with the stored status.
func (r *KubernetesKonnectivityGatewayResource) gatewayStatusNeedsUpdate(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	currentStatus := tcp.Status.Addons.Konnectivity.Gateway

	// Check if route reference has changed
	if currentStatus != nil && currentStatus.RouteRef.Name != r.resource.Name {
		return true
	}

	// Compare RouteStatus - check if number of parents changed
	return resources.IsGatewayRouteStatusChanged(currentStatus, r.resource.Status.RouteStatus)
}

func (r *KubernetesKonnectivityGatewayResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.shouldHaveGateway(tcp) && tcp.Status.Addons.Konnectivity.Gateway != nil
}

func (r *KubernetesKonnectivityGatewayResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	cleaned, err := resources.CleanupTLSRoute(ctx, r.Client, r.resource.GetName(), r.resource.GetNamespace(), tcp)
	if err != nil {
		logger.Error(err, "failed to cleanup konnectivity route")

		return false, err
	}

	if cleaned {
		logger.V(1).Info("konnectivity route cleaned up successfully")
	}

	return cleaned, nil
}

func (r *KubernetesKonnectivityGatewayResource) UpdateTenantControlPlaneStatus(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	// Clean up status if Gateway routes are no longer configured
	if !r.shouldHaveGateway(tcp) {
		tcp.Status.Addons.Konnectivity.Gateway = nil

		return nil
	}

	tcp.Status.Addons.Konnectivity.Gateway = &kamajiv1alpha1.KubernetesGatewayStatus{
		RouteStatus: r.resource.Status.RouteStatus,
		RouteRef: v1.LocalObjectReference{
			Name: r.resource.Name,
		},
	}

	routeStatuses := tcp.Status.Addons.Konnectivity.Gateway.RouteStatus

	// TODO: Investigate the implications of having multiple parents / hostnames
	// TODO: Use condition to report?
	if len(routeStatuses.Parents) == 0 {
		return fmt.Errorf("no gateway attached to the konnectivity route")
	}
	if len(routeStatuses.Parents) > 1 {
		return fmt.Errorf("too many gateways attached to the konnectivity route")
	}
	if len(r.resource.Spec.Hostnames) == 0 {
		return fmt.Errorf("no hostname in the konnectivity route")
	}
	if len(r.resource.Spec.Hostnames) > 1 {
		return fmt.Errorf("too many hostnames in the konnectivity route")
	}

	logger.V(1).Info("updating TenantControlPlane status for Konnectivity Gateway routes")
	accessPoints, err := resources.BuildGatewayAccessPointsStatus(ctx, r.Client, r.resource, routeStatuses)
	if err != nil {
		return err
	}
	tcp.Status.Addons.Konnectivity.Gateway.AccessPoints = accessPoints

	return nil
}

func (r *KubernetesKonnectivityGatewayResource) Define(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-konnectivity", tcp.GetName()),
			Namespace: tcp.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesKonnectivityGatewayResource) mutate(tcp *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		// Use control plane gateway configuration
		if tcp.Spec.ControlPlane.Gateway == nil {
			return fmt.Errorf("control plane gateway is not configured")
		}

		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(tcp.GetName(), r.GetName()),
			tcp.Spec.ControlPlane.Gateway.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tcp.Spec.ControlPlane.Gateway.AdditionalMetadata.Annotations,
		)
		r.resource.SetAnnotations(annotations)

		// Use hostname from control plane gateway
		if len(tcp.Spec.ControlPlane.Gateway.Hostname) == 0 {
			return fmt.Errorf("control plane gateway hostname is not set")
		}

		serviceName := gatewayv1alpha2.ObjectName(tcp.Status.Addons.Konnectivity.Service.Name)
		servicePort := tcp.Status.Addons.Konnectivity.Service.Port

		if serviceName == "" || servicePort == 0 {
			return fmt.Errorf("konnectivity service not ready, cannot create TLSRoute")
		}

		// Copy parentRefs from control plane gateway and explicitly set port and sectionName fields
		if tcp.Spec.ControlPlane.Gateway.GatewayParentRefs == nil {
			return fmt.Errorf("control plane gateway parentRefs are not specified")
		}
		r.resource.Spec.ParentRefs = newParentRefsSpecWithPortAndSection(tcp.Spec.ControlPlane.Gateway.GatewayParentRefs, servicePort, "konnectivity-server")

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

func (r *KubernetesKonnectivityGatewayResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if !r.shouldHaveGateway(tenantControlPlane) {
		return controllerutil.OperationResultNone, nil
	}

	if tenantControlPlane.Spec.ControlPlane.Gateway == nil {
		return controllerutil.OperationResultNone, nil
	}

	if len(tenantControlPlane.Spec.ControlPlane.Gateway.Hostname) == 0 {
		return controllerutil.OperationResultNone, fmt.Errorf("missing hostname to expose Konnectivity using a Gateway resource")
	}

	logger.V(1).Info("creating or updating resource konnectivity gateway routes")

	result, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tenantControlPlane))
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *KubernetesKonnectivityGatewayResource) GetName() string {
	return "konnectivity_gateway_routes"
}

// newParentRefsSpecWithPortAndSection creates a copy of parentRefs with port and sectionName set for each reference.
func newParentRefsSpecWithPortAndSection(parentRefs []gatewayv1.ParentReference, port int32, sectionName string) []gatewayv1.ParentReference {
	result := make([]gatewayv1.ParentReference, len(parentRefs))
	sectionNamePtr := gatewayv1.SectionName(sectionName)
	for i, parentRef := range parentRefs {
		result[i] = *parentRef.DeepCopy()
		result[i].Port = &port
		result[i].SectionName = &sectionNamePtr
	}

	return result
}
