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

type Freeze struct{}

func (f Freeze) OnCreate(runtime.Object) AdmissionResponse {
	return f.response
}

func (f Freeze) OnDelete(runtime.Object) AdmissionResponse {
	return f.response
}

func (f Freeze) OnUpdate(runtime.Object, runtime.Object) AdmissionResponse {
	return f.response
}

func (f Freeze) response(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
	return nil, fmt.Errorf("the current Control Plane is in freezing mode due to a maintenance mode, all the changes are blocked: " +
		"removing the webhook may lead to an inconsistent state upon its completion")
}
