// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestResolveTenantControlPlaneAddressFromAssignedEndpoint(t *testing.T) {
	t.Parallel()

	tcp := &kamajiv1alpha1.TenantControlPlane{
		Status: kamajiv1alpha1.TenantControlPlaneStatus{
			ControlPlaneEndpoint: "203.0.113.10:6443",
		},
	}

	address := resolveTenantControlPlaneAddress(tcp)
	if address != "https://203.0.113.10:6443" {
		t.Fatalf("expected endpoint address https://203.0.113.10:6443, got %q", address)
	}
}

func TestResolveTenantControlPlaneAddressReturnsEmptyWhenUnresolvable(t *testing.T) {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Service: kamajiv1alpha1.ServiceSpec{ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer},
			},
		},
	}

	address := resolveTenantControlPlaneAddress(tcp)
	if address != "" {
		t.Fatalf("expected empty address fallback, got %q", address)
	}
}
