// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (in *TenantControlPlane) GetAddress(ctx context.Context, client client.Client) (string, error) {
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
			return "", fmt.Errorf("cannot retrieve the TenantControlPlane address, Service resource is not yet exposed as LoadBalancer")
		}

		for _, lb := range loadBalancerStatus.Ingress {
			if ip := lb.IP; len(ip) > 0 {
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("the actual resource doesn't have yet a valid IP address")
}
