// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	json "github.com/json-iterator/go"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func JSONPatch(original, modified client.Object) ([]jsonpatch.Operation, error) {
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal original object: %w", err)
	}

	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal modified object: %w", err)
	}

	return jsonpatch.CreatePatch(originalJSON, modifiedJSON)
}
