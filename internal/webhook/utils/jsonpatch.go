// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	json "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func JSONPatch(obj client.Object, modifierFunc func()) ([]jsonpatch.Operation, error) {
	original, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal input object")
	}

	modifierFunc()

	patched, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal patched object")
	}

	return jsonpatch.CreatePatch(original, patched)
}
