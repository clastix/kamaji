// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneGatewayValidation struct {
	GatewayAPIEnabled bool
}

func (t TenantControlPlaneGatewayValidation) OnCreate(object runtime.Object) AdmissionResponse {
	return func(_ context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*kamajiv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		if tcp.Spec.ControlPlane.Gateway != nil {
			if err := t.validateGatewayAPIEnabled(); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}
}

func (t TenantControlPlaneGatewayValidation) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(_ context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*kamajiv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		if tcp.Spec.ControlPlane.Gateway != nil {
			if err := t.validateGatewayAPIEnabled(); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}
}

func (t TenantControlPlaneGatewayValidation) OnDelete(object runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneGatewayValidation) validateGatewayAPIEnabled() error {
	if !t.GatewayAPIEnabled {
		return fmt.Errorf("cannot use the gateway configuration while Gateway API support is disabled: start Kamaji with --enable-gateway-api")
	}

	return nil
}
