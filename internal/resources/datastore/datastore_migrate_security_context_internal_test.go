// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestMigrateSetSecurityContext(t *testing.T) {
	d := &Migrate{
		job: &batchv1.Job{
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "migrate"}}},
				},
			},
		},
	}

	d.setSecurityContext()

	wantPod := &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	if got := d.job.Spec.Template.Spec.SecurityContext; !reflect.DeepEqual(got, wantPod) {
		t.Errorf("pod securityContext = %+v, want %+v", got, wantPod)
	}

	wantContainer := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
	if got := d.job.Spec.Template.Spec.Containers[0].SecurityContext; !reflect.DeepEqual(got, wantContainer) {
		t.Errorf("container securityContext = %+v, want %+v", got, wantContainer)
	}
}
