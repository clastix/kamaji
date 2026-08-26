// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/internal/errors"
)

// AssignedControlPlaneAddress returns the announced address and port of a Tenant Control Plane.
// In case of non-well formed values, or missing announcement, an error is returned.
func (in *TenantControlPlane) AssignedControlPlaneAddress() (string, int32, error) {
	if len(in.Status.ControlPlaneEndpoint) == 0 {
		return "", 0, fmt.Errorf("the Tenant Control Plane is not yet exposed")
	}

	address, portString, err := net.SplitHostPort(in.Status.ControlPlaneEndpoint)
	if err != nil {
		return "", 0, fmt.Errorf("cannot split host port from Tenant Control Plane endpoint: %w", err)
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, fmt.Errorf("cannot convert Tenant Control Plane port from endpoint: %w", err)
	}

	return address, int32(port), nil
}

// AdvertisedControlPlaneAddress returns the address and port to advertise to tenant-side consumers.
// If AdvertiseAddress is set, it is returned with the same port as the management address.
// Otherwise, it falls back to AssignedControlPlaneAddress.
func (in *TenantControlPlane) AdvertisedControlPlaneAddress() (string, int32, error) {
	if in.Spec.NetworkProfile.AdvertiseAddress != "" {
		_, port, err := in.AssignedControlPlaneAddress()
		if err != nil {
			return "", 0, err
		}

		return in.Spec.NetworkProfile.AdvertiseAddress, port, nil
	}

	return in.AssignedControlPlaneAddress()
}

// DeclaredControlPlaneAddress returns the desired Tenant Control Plane address, used as the
// kube-apiserver advertise address and certificate IP SAN. This must always resolve to a real
// IP address (or in-cluster DNS name), never PublicAPIServerAddress: kubeadm requires
// LocalAPIEndpoint.AdvertiseAddress to be a valid IP, and the public hostname is only used
// externally via PublicControlPlaneAddress (cluster-info ConfigMap, admin/kubeconfig Server).
// For services, it returns clusterIP if available, otherwise the DNS name.
// When the address cannot be determined, an error is returned.
func (in *TenantControlPlane) DeclaredControlPlaneAddress(ctx context.Context, client client.Client) (string, error) {
	switch {
	case len(in.Spec.NetworkProfile.Address) > 0:
		// Returning the hard-coded value in the specification in case of non LoadBalanced resources
		return in.Spec.NetworkProfile.Address, nil
	default:
		// Try to get the service clusterIP, otherwise use DNS name
		svc := &corev1.Service{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: in.GetNamespace(), Name: in.GetName()}, svc); err != nil {
			return "", fmt.Errorf("cannot retrieve Service for the TenantControlPlane: %w", err)
		}
		if len(svc.Spec.ClusterIP) > 0 {
			return svc.Spec.ClusterIP, nil
		}
		// Fall back to DNS name if clusterIP is not yet assigned
		return fmt.Sprintf("%s.%s.svc", in.GetName(), in.GetNamespace()), nil
	}

	return "", errors.MissingValidIPError{}
}

// ControlPlaneServiceIPs returns every IP address the Tenant Control Plane Service
// answers on: all of its ClusterIPs (covering both families of a dual-stack Service)
// and all LoadBalancer ingress IPs. It is meant for certificate SANs, so it is
// best-effort about IP availability: a not-yet-provisioned LoadBalancer simply yields
// no ingress IPs rather than an error. A missing Service, in contrast, is returned as
// an error. The caller is expected to also include the primary advertised/management
// address.
func (in *TenantControlPlane) ControlPlaneServiceIPs(ctx context.Context, c client.Client) ([]string, error) {
	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: in.GetNamespace(), Name: in.GetName()}, svc); err != nil {
		return nil, fmt.Errorf("cannot retrieve Service for the TenantControlPlane: %w", err)
	}

	// ClusterIPs carries both families of a dual-stack Service; fall back to the
	// singular ClusterIP for Services that only populate the legacy field.
	clusterIPs := svc.Spec.ClusterIPs
	if len(clusterIPs) == 0 && len(svc.Spec.ClusterIP) > 0 {
		clusterIPs = []string{svc.Spec.ClusterIP}
	}

	ips := make([]string, 0, len(clusterIPs)+len(svc.Status.LoadBalancer.Ingress))

	for _, ip := range clusterIPs {
		if ip == "" || ip == corev1.ClusterIPNone {
			continue
		}

		ips = append(ips, ip)
	}

	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if len(ingress.IP) > 0 {
			ips = append(ips, ingress.IP)
		}
	}

	return ips, nil
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

	return "", errors.MissingValidIPError{}
}

func (in *TenantControlPlane) GetDefaultDatastoreUsername() string {
	return string(in.UID)
}

func (in *TenantControlPlane) GetDefaultDatastoreSchema() string {
	return string(in.UID)
}
