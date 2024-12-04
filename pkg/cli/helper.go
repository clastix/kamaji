// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Helper struct {
	Client client.Client
}
