// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneDefaults struct {
	DefaultDatastore      string
	DefaultServiceAccount string
}

func (t TenantControlPlaneDefaults) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if tcp.Spec.ControlPlane.Deployment.Replicas == nil {
			tcp.Spec.ControlPlane.Deployment.Replicas = pointer.To(int32(2))
		}

		addDatastore := len(tcp.Spec.DataStore) == 0
		addServiceAccount := len(tcp.Spec.ControlPlane.Deployment.ServiceAccountName) == 0

		var operations []jsonpatch.Operation
		var err error

		if addDatastore && addServiceAccount {
			operations, err = utils.JSONPatch(tcp, func() {
				tcp.Spec.DataStore = t.DefaultDatastore
				tcp.Spec.ControlPlane.Deployment.ServiceAccountName = t.DefaultServiceAccount
			})
		} else if addDatastore {
			operations, err = utils.JSONPatch(tcp, func() {
				tcp.Spec.DataStore = t.DefaultDatastore
			})
		} else {
			operations, err = utils.JSONPatch(tcp, func() {
				tcp.Spec.ControlPlane.Deployment.ServiceAccountName = t.DefaultServiceAccount
			})
		}
		if err != nil {
			return nil, errors.Wrap(err, "cannot create patch responses upon Tenant Control Plane creation")
		}

		return operations, nil
	}
}

func (t TenantControlPlaneDefaults) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneDefaults) OnUpdate(object runtime.Object, oldObject runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		newTCP, oldTCP := object.(*kamajiv1alpha1.TenantControlPlane), oldObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if oldTCP.Spec.DataStore == newTCP.Spec.DataStore {
			return nil, nil
		}

		if len(newTCP.Spec.DataStore) == 0 {
			return nil, fmt.Errorf("DataStore is a required field")
		}

		if oldTCP.Spec.ControlPlane.Deployment.ServiceAccountName != newTCP.Spec.ControlPlane.Deployment.ServiceAccountName {
			return nil, fmt.Errorf("changing serviceAccountName is not supported")
		}

		if newTCP.Spec.ControlPlane.Deployment.Replicas == nil {
			newTCP.Spec.ControlPlane.Deployment.Replicas = pointer.To(int32(2))
		}

		return nil, nil
	}
}
