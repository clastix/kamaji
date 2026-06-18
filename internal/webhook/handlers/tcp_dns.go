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
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneDNS struct{}

func (t TenantControlPlaneDNS) OnCreate(object runtime.Object) AdmissionResponse {
	return t.validate(object)
}

func (t TenantControlPlaneDNS) OnUpdate(_, newObject runtime.Object) AdmissionResponse {
	return t.validate(newObject)
}

func (t TenantControlPlaneDNS) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneDNS) validate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if err := validateDNSServiceIPs(tcp); err != nil {
			return nil, err
		}

		return nil, nil
	}
}

func validateDNSServiceIPs(tcp *kamajiv1alpha1.TenantControlPlane) error {
	if len(tcp.Spec.NetworkProfile.DNSServiceIPs) == 0 {
		return nil
	}

	serviceCIDRs := tcp.Spec.NetworkProfile.ServiceCIDRs

	// Backward compatibility
	if len(serviceCIDRs) == 0 && tcp.Spec.NetworkProfile.ServiceCIDR != "" {
		serviceCIDRs = []string{tcp.Spec.NetworkProfile.ServiceCIDR}
	}

	// Safety check
	if len(serviceCIDRs) == 0 {
		return fmt.Errorf("no service CIDRs defined")
	}

	for _, dnsIP := range tcp.Spec.NetworkProfile.DNSServiceIPs {
		parsedIP := net.ParseIP(dnsIP)
		if parsedIP == nil {
			return fmt.Errorf("DNS service IP %q is not a valid IP address", dnsIP)
		}

		found := false

		for _, cidr := range serviceCIDRs {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				return fmt.Errorf("invalid service CIDR %q: %w", cidr, err)
			}

			if network.Contains(parsedIP) {
				found = true

				break
			}
		}

		if !found {
			return fmt.Errorf(
				"DNS service IP %q is not contained in any configured service CIDR",
				dnsIP,
			)
		}
	}

	return nil
}
