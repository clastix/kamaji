// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// PausedReconciliationAnnotation is an annotation that can be applied to
	// Tenant Control Plane objects to prevent the controller from processing such a resource.
	PausedReconciliationAnnotation = "kamaji.clastix.io/paused"

	// DefaultKubernetesVersion is the default Kubernetes version used in e2e tests.
	DefaultKubernetesVersion = "v1.37.0"

	// TenantControlPlaneConditionReadyType is the standard Ready condition type: it is intentionally
	// not prefixed, since generic tooling (kubectl wait, kstatus) looks for a condition named Ready.
	TenantControlPlaneConditionReadyType = "Ready"
)
