// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajierrors "github.com/clastix/kamaji/internal/errors"
)

func (in *TenantControlPlane) GetControlPlaneAddress(ctx context.Context, client client.Client) (string, error) {
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
