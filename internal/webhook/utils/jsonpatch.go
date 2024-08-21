// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	json "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func JSONPatch(original, modified client.Object) ([]jsonpatch.Operation, error) {
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal original object")
	}

	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal modified object")
	}

	return jsonpatch.CreatePatch(originalJSON, modifiedJSON)
}
