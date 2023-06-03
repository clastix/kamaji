// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-datastore,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=datastores,verbs=create;update;delete,versions=v1alpha1,name=vdatastore.kb.io,admissionReviewVersions=v1

type DataStoreValidate struct{}

func (d DataStoreValidate) GetPath() string {
	return "/validate-kamaji-clastix-io-v1alpha1-datastore"
}

func (d DataStoreValidate) GetObject() runtime.Object {
	return &kamajiv1alpha1.DataStore{}
}
