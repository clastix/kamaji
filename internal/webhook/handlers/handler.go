// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type AdmissionResponse func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error)

type Handler interface {
	OnCreate(obj runtime.Object) AdmissionResponse
	OnDelete(obj runtime.Object) AdmissionResponse
	OnUpdate(newObject runtime.Object, prevObject runtime.Object) AdmissionResponse
}
