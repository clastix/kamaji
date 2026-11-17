// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetReadyCondition derives the standard Ready condition from the Kubernetes version status,
// which remains the source of truth: the condition is only its translation for generic tooling.
//
// The condition reports the last fully reconciled generation (Status.ObservedGeneration), not the
// current one: status writes performed mid-reconciliation of a new generation must not stamp the
// previous readiness as current. Consumers (kubectl wait, kstatus) then see the condition lagging
// behind metadata.generation until the reconciliation is complete.
func (in *TenantControlPlane) SetReadyCondition() {
	condition := metav1.Condition{
		Type:               TenantControlPlaneConditionReadyType,
		ObservedGeneration: in.Status.ObservedGeneration,
	}

	switch status := in.Status.Kubernetes.Version.Status; {
	case status == nil:
		condition.Status = metav1.ConditionUnknown
		condition.Reason = string(VersionUnknown)
		condition.Message = "the Tenant Control Plane has not been reconciled yet"
	case *status == VersionReady:
		condition.Status = metav1.ConditionTrue
		condition.Reason = string(*status)
		condition.Message = "the Tenant Control Plane is ready"
	case *status == VersionWriteLimited:
		condition.Status = metav1.ConditionTrue
		condition.Reason = string(*status)
		condition.Message = "the Tenant Control Plane is serving, with limited write permissions"
	default:
		condition.Status = metav1.ConditionFalse
		condition.Reason = string(*status)
		condition.Message = fmt.Sprintf("the Tenant Control Plane is in %s status", *status)
	}

	meta.SetStatusCondition(&in.Status.Conditions, condition)
}
