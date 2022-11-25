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
var datastorelog = logf.Log.WithName("datastore-resource")

func (r *DataStore) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-kamaji-clastix-io-v1alpha1-datastore,mutating=true,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=datastores,verbs=create;update,versions=v1alpha1,name=mdatastore.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &DataStore{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *DataStore) Default() {
	datastorelog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-datastore,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=datastores,verbs=create;update,versions=v1alpha1,name=vdatastore.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DataStore{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *DataStore) ValidateCreate() error {
	datastorelog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *DataStore) ValidateUpdate(old runtime.Object) error {
	datastorelog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *DataStore) ValidateDelete() error {
	datastorelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
