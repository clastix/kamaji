// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestMigrateSetSecurityContext(t *testing.T) {
	defaultPod := &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	defaultContainer := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
	customPod := &corev1.PodSecurityContext{
		RunAsUser:  ptr.To[int64](1000),
		RunAsGroup: ptr.To[int64](1000),
		FSGroup:    ptr.To[int64](1000),
	}
	customContainer := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: ptr.To(true),
		RunAsNonRoot:           ptr.To(true),
	}

	tests := []struct {
		name          string
		migration     *kamajiv1alpha1.DataStoreMigration
		wantPod       *corev1.PodSecurityContext
		wantContainer *corev1.SecurityContext
	}{
		{
			name:          "no migration settings use the restricted defaults",
			wantPod:       defaultPod,
			wantContainer: defaultContainer,
		},
		{
			name:          "unset security contexts use the restricted defaults",
			migration:     &kamajiv1alpha1.DataStoreMigration{NodeSelector: map[string]string{"kubernetes.io/os": "linux"}},
			wantPod:       defaultPod,
			wantContainer: defaultContainer,
		},
		{
			name: "set security contexts replace the defaults",
			migration: &kamajiv1alpha1.DataStoreMigration{
				PodSecurityContext: customPod,
				SecurityContext:    customContainer,
			},
			wantPod:       customPod,
			wantContainer: customContainer,
		},
		{
			name:          "only the pod security context is set",
			migration:     &kamajiv1alpha1.DataStoreMigration{PodSecurityContext: customPod},
			wantPod:       customPod,
			wantContainer: defaultContainer,
		},
		{
			name:          "only the container security context is set",
			migration:     &kamajiv1alpha1.DataStoreMigration{SecurityContext: customContainer},
			wantPod:       defaultPod,
			wantContainer: customContainer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Migrate{
				desiredDatastore: &kamajiv1alpha1.DataStore{
					Spec: kamajiv1alpha1.DataStoreSpec{Migration: tt.migration},
				},
				job: &batchv1.Job{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "migrate"}}},
						},
					},
				},
			}

			d.setSecurityContext()

			if got := d.job.Spec.Template.Spec.SecurityContext; !reflect.DeepEqual(got, tt.wantPod) {
				t.Errorf("pod securityContext = %+v, want %+v", got, tt.wantPod)
			}
			if got := d.job.Spec.Template.Spec.Containers[0].SecurityContext; !reflect.DeepEqual(got, tt.wantContainer) {
				t.Errorf("container securityContext = %+v, want %+v", got, tt.wantContainer)
			}
		})
	}
}
