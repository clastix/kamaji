// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type TenantControlPlaneRetrievalFn func() (*kamajiv1alpha1.TenantControlPlane, error)
