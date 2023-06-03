// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type TenantControlPlaneMigrate struct{}

func (t TenantControlPlaneMigrate) GetPath() string {
	return "/migrate"
}

func (t TenantControlPlaneMigrate) GetObject() runtime.Object {
	return &kamajiv1alpha1.TenantControlPlane{}
}
