// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestSetReadyCondition(t *testing.T) {
	tests := []struct {
		name           string
		status         *KubernetesVersionStatus
		expectedStatus metav1.ConditionStatus
		expectedReason string
	}{
		{name: "not reconciled yet", status: nil, expectedStatus: metav1.ConditionUnknown, expectedReason: "Unknown"},
		{name: "ready", status: ptr.To(VersionReady), expectedStatus: metav1.ConditionTrue, expectedReason: "Ready"},
		{name: "write limited", status: ptr.To(VersionWriteLimited), expectedStatus: metav1.ConditionTrue, expectedReason: "WriteLimited"},
		{name: "provisioning", status: ptr.To(VersionProvisioning), expectedStatus: metav1.ConditionFalse, expectedReason: "Provisioning"},
		{name: "upgrading", status: ptr.To(VersionUpgrading), expectedStatus: metav1.ConditionFalse, expectedReason: "Upgrading"},
		{name: "migrating", status: ptr.To(VersionMigrating), expectedStatus: metav1.ConditionFalse, expectedReason: "Migrating"},
		{name: "ca rotating", status: ptr.To(VersionCARotating), expectedStatus: metav1.ConditionFalse, expectedReason: "CertificateAuthorityRotating"},
		{name: "sleeping", status: ptr.To(VersionSleeping), expectedStatus: metav1.ConditionFalse, expectedReason: "Sleeping"},
		{name: "not ready", status: ptr.To(VersionNotReady), expectedStatus: metav1.ConditionFalse, expectedReason: "NotReady"},
		{name: "unknown", status: ptr.To(VersionUnknown), expectedStatus: metav1.ConditionFalse, expectedReason: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcp := &TenantControlPlane{ObjectMeta: metav1.ObjectMeta{Generation: 3}}
			tcp.Status.Kubernetes.Version.Status = tt.status

			tcp.SetReadyCondition()

			condition := meta.FindStatusCondition(tcp.Status.Conditions, TenantControlPlaneConditionReadyType)
			if condition == nil {
				t.Fatalf("expected a %s condition, got none", TenantControlPlaneConditionReadyType)
			}

			if condition.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, condition.Status)
			}

			if condition.Reason != tt.expectedReason {
				t.Errorf("expected reason %s, got %s", tt.expectedReason, condition.Reason)
			}

			if condition.ObservedGeneration != 3 {
				t.Errorf("expected observedGeneration 3, got %d", condition.ObservedGeneration)
			}

			if condition.Message == "" {
				t.Error("expected a non-empty message")
			}
		})
	}
}

func TestSetReadyConditionTransitions(t *testing.T) {
	tcp := &TenantControlPlane{}
	tcp.Status.Kubernetes.Version.Status = ptr.To(VersionProvisioning)
	tcp.SetReadyCondition()

	past := metav1.NewTime(time.Now().Add(-time.Hour))
	tcp.Status.Conditions[0].LastTransitionTime = past

	// Same status: the transition time must not move, otherwise watchers see spurious changes.
	tcp.SetReadyCondition()

	if got := tcp.Status.Conditions[0].LastTransitionTime; !got.Equal(&past) {
		t.Errorf("expected lastTransitionTime to be preserved, got %s", got)
	}

	// Status flip: the transition time must be updated, and only one Ready condition must exist.
	tcp.Status.Kubernetes.Version.Status = ptr.To(VersionReady)
	tcp.SetReadyCondition()

	if len(tcp.Status.Conditions) != 1 {
		t.Fatalf("expected exactly one condition, got %d", len(tcp.Status.Conditions))
	}

	if got := tcp.Status.Conditions[0]; got.Status != metav1.ConditionTrue || got.LastTransitionTime.Equal(&past) {
		t.Errorf("expected a True condition with a new lastTransitionTime, got %+v", got)
	}
}
