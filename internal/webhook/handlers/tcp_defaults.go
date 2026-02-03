// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"net"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneDefaults struct {
	DefaultDatastore string
}

func (t TenantControlPlaneDefaults) OnCreate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		original := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		defaulted := original.DeepCopy()
		t.defaultUnsetFields(defaulted)

		if len(defaulted.Spec.NetworkProfile.DNSServiceIPs) == 0 {
			ip, _, err := net.ParseCIDR(defaulted.Spec.NetworkProfile.ServiceCIDR)
			if err != nil {
				return nil, fmt.Errorf("cannot define resulting DNS Service IP: %w", err)
			}
			switch {
			case ip.To4() != nil:
				ip[len(ip)-1] += 10
			case ip.To16() != nil:
				ip[len(ip)-1] += 16
			}

			defaulted.Spec.NetworkProfile.DNSServiceIPs = []string{ip.String()}
		}

		operations, err := utils.JSONPatch(original, defaulted)
		if err != nil {
			return nil, fmt.Errorf("cannot create patch responses upon Tenant Control Plane creation: %w", err)
		}

		return operations, nil
	}
}

func (t TenantControlPlaneDefaults) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneDefaults) OnUpdate(runtime.Object, runtime.Object) AdmissionResponse {
	// all immutability requirements are handled trough CEL annotations on the TenantControlPlaneSpec type
	return utils.NilOp()
}

func (t TenantControlPlaneDefaults) defaultUnsetFields(tcp *kamajiv1alpha1.TenantControlPlane) {
	if len(tcp.Spec.DataStore) == 0 && t.DefaultDatastore != "" {
		tcp.Spec.DataStore = t.DefaultDatastore
	}

	if tcp.Spec.ControlPlane.Deployment.Replicas == nil {
		tcp.Spec.ControlPlane.Deployment.Replicas = pointer.To(int32(2))
	}

	if len(tcp.Spec.DataStoreSchema) == 0 {
		tcp.Spec.DataStoreSchema = tcp.GetDefaultDatastoreSchema()
	}

	if len(tcp.Spec.DataStoreUsername) == 0 {
		tcp.Spec.DataStoreUsername = tcp.GetDefaultDatastoreUsername()
	}
}
