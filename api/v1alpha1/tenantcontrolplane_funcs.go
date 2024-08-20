// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajierrors "github.com/clastix/kamaji/internal/errors"
)

// AssignedControlPlaneAddress returns the announced address and port of a Tenant Control Plane.
// In case of non-well formed values, or missing announcement, an error is returned.
func (in *TenantControlPlane) AssignedControlPlaneAddress() (string, int32, error) {
	if len(in.Status.ControlPlaneEndpoint) == 0 {
		return "", 0, fmt.Errorf("the Tenant Control Plane is not yet exposed")
	}

	address, portString, err := net.SplitHostPort(in.Status.ControlPlaneEndpoint)
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot split host port from Tenant Control Plane endpoint")
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot convert Tenant Control Plane port from endpoint")
	}

	return address, int32(port), nil
}

// DeclaredControlPlaneAddress returns the desired Tenant Control Plane address.
// In case of dynamic allocation, e.g. using a Load Balancer, it queries the API Server looking for the allocated IP.
// When an IP has not been yet assigned, or it is expected, an error is returned.
func (in *TenantControlPlane) DeclaredControlPlaneAddress(ctx context.Context, client client.Client) (string, error) {
	var loadBalancerStatus corev1.LoadBalancerStatus

	svc := &corev1.Service{}
	err := client.Get(ctx, types.NamespacedName{Namespace: in.GetNamespace(), Name: in.GetName()}, svc)
	if err != nil {
		return "", errors.Wrap(err, "cannot retrieve Service for the TenantControlPlane")
	}

	switch {
	case len(in.Spec.NetworkProfile.Address) > 0:
		// Returning the hard-coded value in the specification in case of non LoadBalanced resources
		return in.Spec.NetworkProfile.Address, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP:
		return svc.Spec.ClusterIP, nil
	case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
		loadBalancerStatus = svc.Status.LoadBalancer
		if len(loadBalancerStatus.Ingress) == 0 {
			return "", kamajierrors.NonExposedLoadBalancerError{}
		}

		return getLoadBalancerAddress(loadBalancerStatus.Ingress)
	}

	return "", kamajierrors.MissingValidIPError{}
}

// getLoadBalancerAddress extracts the IP address from LoadBalancer ingress.
// It also checks and rejects hostname usage for LoadBalancer ingress.
//
// Reasons for not supporting hostnames:
// - DNS resolution can differ across environments, leading to inconsistent behavior.
// - It may cause connectivity problems between Kubernetes components.
// - The DNS resolution could change over time, potentially breaking cluster-to-API-server connections.
//
// Recommended solutions:
// - Use a static IP address to ensure stable and predictable communication within the cluster.
// - If a hostname is necessary, consider setting up a Virtual IP (VIP) for the given hostname.
// - Alternatively, use an external load balancer that can provide a stable IP address.
//
// Note: Implementing L7 routing with the API Server requires a deep understanding of the implications.
// Users should be aware of the complexities involved, including potential issues with TLS passthrough
// for client-based certificate authentication in Ingress expositions.
func getLoadBalancerAddress(ingress []corev1.LoadBalancerIngress) (string, error) {
	for _, lb := range ingress {
		if ip := lb.IP; len(ip) > 0 {
			return ip, nil
		}
		if hostname := lb.Hostname; len(hostname) > 0 {
			return "", fmt.Errorf("hostname not supported for LoadBalancer ingress: use static IP instead")
		}
	}

	return "", kamajierrors.MissingValidIPError{}
}
