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

type TenantControlPlaneServiceCIDR struct{}

func (t TenantControlPlaneServiceCIDR) handle(tcp *kamajiv1alpha1.TenantControlPlane) error {
	if tcp.Spec.Addons.CoreDNS == nil {
		return nil
	}

	_, cidr, err := net.ParseCIDR(tcp.Spec.NetworkProfile.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("unable to parse Service CIDR, %s", err.Error())
	}

	for _, serviceIP := range tcp.Spec.NetworkProfile.DNSServiceIPs {
		ip := net.ParseIP(serviceIP)
		if ip == nil {
			return fmt.Errorf("unable to parse IP address %s", serviceIP)
		}

		if !cidr.Contains(ip) {
			return fmt.Errorf("the Service CIDR does not contain the DNS Service IP %s", serviceIP)
		}
	}

	return nil
}

func (t TenantControlPlaneServiceCIDR) OnCreate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if err := t.handle(tcp); err != nil {
			return nil, err
		}

		return nil, nil
	}
}

func (t TenantControlPlaneServiceCIDR) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneServiceCIDR) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if err := t.handle(tcp); err != nil {
			return nil, err
		}

		return nil, nil
	}
}
