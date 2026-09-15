// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"net"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type TenantControlPlaneValidateCIDRs struct{}

func (h TenantControlPlaneValidateCIDRs) OnCreate(obj runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := obj.(*kamajiv1alpha1.TenantControlPlane)
		if !ok {
			return nil, nil
		}

		return nil, h.validateCIDRs(tcp)
	}
}

func (h TenantControlPlaneValidateCIDRs) OnDelete(obj runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		return nil, nil
	}
}

func (h TenantControlPlaneValidateCIDRs) OnUpdate(newObject runtime.Object, prevObject runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := newObject.(*kamajiv1alpha1.TenantControlPlane)
		if !ok {
			return nil, nil
		}

		return nil, h.validateCIDRs(tcp)
	}
}

func (h TenantControlPlaneValidateCIDRs) validateCIDRs(tcp *kamajiv1alpha1.TenantControlPlane) error {
	// Validate ServiceCIDRs
	if len(tcp.Spec.NetworkProfile.ServiceCIDRs) > 1 {
		if h.hasSameFamilyCIDRs(tcp.Spec.NetworkProfile.ServiceCIDRs) {
			return fmt.Errorf("serviceCidrs must not contain two CIDRs of the same IP family")
		}
	}

	// Validate PodCIDRs
	if len(tcp.Spec.NetworkProfile.PodCIDRs) > 1 {
		if h.hasSameFamilyCIDRs(tcp.Spec.NetworkProfile.PodCIDRs) {
			return fmt.Errorf("podCidrs must not contain two CIDRs of the same IP family")
		}
	}

	// Validate PreferredAddressTypes uniqueness
	seen := make(map[kamajiv1alpha1.KubeletPreferredAddressType]bool)
	for _, addrType := range tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes {
		if seen[addrType] {
			return fmt.Errorf("preferredAddressTypes entries must be unique")
		}
		seen[addrType] = true
	}

	return nil
}

// hasSameFamilyCIDRs checks if CIDRs contain two or more of the same IP family
func (h TenantControlPlaneValidateCIDRs) hasSameFamilyCIDRs(cidrs []string) bool {
	if len(cidrs) < 2 {
		return false
	}

	families := make([]int, 0, len(cidrs))
	for _, cidr := range cidrs {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue // Invalid CIDR will be caught by other validation
		}

		if ip.To4() != nil {
			families = append(families, 4)
		} else if ip.To16() != nil {
			families = append(families, 6)
		}
	}

	// Check if all CIDRs are of the same family
	if len(families) >= 2 {
		allSame := true
		for i := 1; i < len(families); i++ {
			if families[i] != families[0] {
				allSame = false
				break
			}
		}
		return allSame
	}

	return false
}
