// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type TenantControlPlaneWritePermission struct{}

func (t TenantControlPlaneWritePermission) GetPath() string {
	return "/write-permission"
}

func (t TenantControlPlaneWritePermission) GetObject() runtime.Object {
	return nil
}
