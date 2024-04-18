// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//+kubebuilder:webhook:path=/validate--v1-secret,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",resources=secrets,verbs=delete,versions=v1,name=vdatastoresecrets.kb.io,admissionReviewVersions=v1

type DataStoreSecrets struct{}

func (d DataStoreSecrets) GetPath() string {
	return "/validate--v1-secret"
}

func (d DataStoreSecrets) GetObject() runtime.Object {
	return &corev1.Secret{}
}
