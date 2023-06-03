// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=vtenantcontrolplane.kb.io,admissionReviewVersions=v1

type TenantControlPlaneValidate struct{}

func (t TenantControlPlaneValidate) GetPath() string {
	return "/validate-kamaji-clastix-io-v1alpha1-tenantcontrolplane"
}

func (t TenantControlPlaneValidate) GetObject() runtime.Object {
	return &kamajiv1alpha1.TenantControlPlane{}
}
