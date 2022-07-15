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

		for _, lb := range loadBalancerStatus.Ingress {
			if ip := lb.IP; len(ip) > 0 {
				return ip, nil
			}
		}
	}

	return "", kamajierrors.MissingValidIPError{}
}

func (in *TenantControlPlane) GetKonnectivityServerAddress(ctx context.Context, client client.Client) (string, error) {
	var loadBalancerStatus corev1.LoadBalancerStatus

	svc := &corev1.Service{}
	err := client.Get(ctx, types.NamespacedName{Namespace: in.GetNamespace(), Name: in.Status.Addons.Konnectivity.Service.Name}, svc)
	if err != nil {
		return "", errors.Wrap(err, "cannot retrieve Service for Konnectivity")
	}

	switch {
	case len(in.Spec.Addons.Konnectivity.ProxyHost) > 0:
		// Returning the hard-coded value in the specification in case of non LoadBalanced resources
		return in.Spec.Addons.Konnectivity.ProxyHost, nil
	case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
		loadBalancerStatus = svc.Status.LoadBalancer
		if len(loadBalancerStatus.Ingress) == 0 {
			return "", kamajierrors.NonExposedLoadBalancerError{}
		}

		for _, lb := range loadBalancerStatus.Ingress {
			if ip := lb.IP; len(ip) > 0 {
				return ip, nil
			}
		}
	}

	return "", kamajierrors.MissingValidIPError{}
}
