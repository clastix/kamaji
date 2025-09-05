// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type TenantControlPlaneReadOnly struct{}

func (t TenantControlPlaneReadOnly) GetPath() string {
	return "/readonly"
}

func (t TenantControlPlaneReadOnly) GetObject() runtime.Object {
	return nil
}
