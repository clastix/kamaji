// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

//+kubebuilder:webhook:path=/mutate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=mtenantcontrolplane.kb.io,admissionReviewVersions=v1

type TenantControlPlaneDefaults struct{}

func (t TenantControlPlaneDefaults) GetObject() runtime.Object {
	return &kamajiv1alpha1.TenantControlPlane{}
}

func (t TenantControlPlaneDefaults) GetPath() string {
	return "/mutate-kamaji-clastix-io-v1alpha1-tenantcontrolplane"
}
