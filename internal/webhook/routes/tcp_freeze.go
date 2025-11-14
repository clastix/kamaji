// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type TenantControlPlaneMigrate struct{}

func (t TenantControlPlaneMigrate) GetPath() string {
	return "/migrate"
}

func (t TenantControlPlaneMigrate) GetObject() runtime.Object {
	return nil
}
