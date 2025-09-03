// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type WritePermission struct{}

func (f WritePermission) OnCreate(runtime.Object) AdmissionResponse {
	return f.response
}

func (f WritePermission) OnDelete(runtime.Object) AdmissionResponse {
	return f.response
}

func (f WritePermission) OnUpdate(runtime.Object, runtime.Object) AdmissionResponse {
	return f.response
}

func (f WritePermission) response(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
	return nil, fmt.Errorf("the current Control Plane has limited write permissions, current changes are blocked: " +
		"removing the webhook may lead to an inconsistent state upon its completion")
}
