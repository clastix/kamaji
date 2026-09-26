// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneGatewayValidation struct {
	Client          client.Client
	DiscoveryClient discovery.DiscoveryInterface
	// GatewayAPIAvailable is the value the controller resolved at startup: admitting a Gateway spec
	// it will not reconcile would fail silently.
	GatewayAPIAvailable bool
}

func (t TenantControlPlaneGatewayValidation) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*kamajiv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		if tcp.Spec.ControlPlane.Gateway != nil {
			// NOTE: Do we actually want to deny here if Gateway API is not available or a warning?
			// Seems sensible to deny to avoid anything.
			if err := t.validateGatewayAPIAvailability(ctx); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}
}

func (t TenantControlPlaneGatewayValidation) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*kamajiv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		if tcp.Spec.ControlPlane.Gateway != nil {
			if err := t.validateGatewayAPIAvailability(ctx); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}
}

func (t TenantControlPlaneGatewayValidation) OnDelete(object runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneGatewayValidation) validateGatewayAPIAvailability(ctx context.Context) error {
	if !t.GatewayAPIAvailable {
		return fmt.Errorf("the Gateway API was not available when Kamaji started, cannot use gatewayRoute configuration: install the Gateway API CRDs and restart Kamaji")
	}

	// Additional check for TLSRoute specifically
	if !utilities.IsTLSRouteAvailable(ctx, t.Client, t.DiscoveryClient) {
		return fmt.Errorf("TLSRoute resource is not available in this cluster")
	}

	return nil
}
