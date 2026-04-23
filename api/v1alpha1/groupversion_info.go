// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 contains API Schema definitions for the kamaji v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=kamaji.clastix.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "kamaji.clastix.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(GroupVersion,
			&DataStore{}, &DataStoreList{},
			&TenantControlPlane{}, &TenantControlPlaneList{},
			&KubeconfigGenerator{}, &KubeconfigGeneratorList{},
		)

		metav1.AddToGroupVersion(scheme, GroupVersion)

		return nil
	})

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
