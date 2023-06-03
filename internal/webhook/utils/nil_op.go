// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"

	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NilOp() func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		return nil, nil
	}
}
