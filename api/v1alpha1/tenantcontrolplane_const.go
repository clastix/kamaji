// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// PausedReconciliationAnnotation is an annotation that can be applied to
	// Tenant Control Plane objects to prevent the controller from processing such a resource.
	PausedReconciliationAnnotation = "kamaji.clastix.io/paused"
)
