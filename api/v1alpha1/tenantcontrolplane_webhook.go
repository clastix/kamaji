// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var tenantcontrolplanelog = logf.Log.WithName("tenantcontrolplane-resource")

func (in *TenantControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=mtenantcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &TenantControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (in *TenantControlPlane) Default() {
	tenantcontrolplanelog.Info("default", "name", in.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=vtenantcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &TenantControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (in *TenantControlPlane) ValidateCreate() error {
	tenantcontrolplanelog.Info("validate create", "name", in.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (in *TenantControlPlane) ValidateUpdate(old runtime.Object) error {
	tenantcontrolplanelog.Info("validate update", "name", in.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (in *TenantControlPlane) ValidateDelete() error {
	tenantcontrolplanelog.Info("validate delete", "name", in.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
