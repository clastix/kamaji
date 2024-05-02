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
)

type DatastoreNatsValidation struct{}

func (d DatastoreNatsValidation) OnCreate(runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		return nil, nil
	}
}

func (d DatastoreNatsValidation) OnDelete(runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		return nil, nil
	}
}

func (d DatastoreNatsValidation) OnUpdate(newObject runtime.Object, prevObject runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		newDs, oldDs := newObject.(*kamajiv1alpha1.DataStore), prevObject.(*kamajiv1alpha1.DataStore) //nolint:forcetypeassert

		if oldDs.Spec.Driver == kamajiv1alpha1.KineNatsDriver || newDs.Spec.Driver == kamajiv1alpha1.KineNatsDriver {
			// If the NATS Datastore is already used by a Tenant Control Plane
			// and a new one is reclaiming it, we need to stop it, since it's not allowed.
			// TODO(prometherion): remove this after multi-tenancy is implemented for NATS.
			if len(oldDs.Status.UsedBy) > 0 && len(newDs.Status.UsedBy) > 1 {
				return nil, fmt.Errorf("multi-tenancy for NATS Datastore is not yet supported")
			}
		}

		return nil, nil
	}
}
