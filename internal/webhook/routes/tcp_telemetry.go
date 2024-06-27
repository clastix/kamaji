// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

//+kubebuilder:webhook:path=/telemetry,mutating=false,failurePolicy=ignore,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update;delete,versions=v1alpha1,name=telemetry.kamaji.clastix.io,admissionReviewVersions=v1

type TenantControlPlaneTelemetry struct{}

func (t TenantControlPlaneTelemetry) GetPath() string {
	return "/telemetry"
}

func (t TenantControlPlaneTelemetry) GetObject() runtime.Object {
	return &kamajiv1alpha1.TenantControlPlane{}
}
