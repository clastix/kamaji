// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func newSchedulingMigration() *kamajiv1alpha1.DataStoreMigration {
	return &kamajiv1alpha1.DataStoreMigration{
		NodeSelector: map[string]string{"node-role.kubernetes.io/control-plane": ""},
		Tolerations: []corev1.Toleration{{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}},
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "kubernetes.io/arch",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						}},
					}},
				},
			},
		},
	}
}

func TestMigrateSetScheduling(t *testing.T) {
	tests := []struct {
		name      string
		migration *kamajiv1alpha1.DataStoreMigration
		job       corev1.PodSpec
		want      *kamajiv1alpha1.DataStoreMigration
	}{
		{
			name: "no migration settings add no constraints to the Job",
			want: &kamajiv1alpha1.DataStoreMigration{},
		},
		{
			name:      "empty migration settings add no constraints to the Job",
			migration: &kamajiv1alpha1.DataStoreMigration{},
			want:      &kamajiv1alpha1.DataStoreMigration{},
		},
		{
			name:      "migration settings are copied to the Job",
			migration: newSchedulingMigration(),
			want:      newSchedulingMigration(),
		},
		{
			name: "only the given settings are copied to the Job",
			migration: &kamajiv1alpha1.DataStoreMigration{
				Tolerations: newSchedulingMigration().Tolerations,
			},
			want: &kamajiv1alpha1.DataStoreMigration{
				Tolerations: newSchedulingMigration().Tolerations,
			},
		},
		{
			name: "removed migration settings are cleared from the Job",
			job: corev1.PodSpec{
				NodeSelector: newSchedulingMigration().NodeSelector,
				Tolerations:  newSchedulingMigration().Tolerations,
				Affinity:     newSchedulingMigration().Affinity,
			},
			want: &kamajiv1alpha1.DataStoreMigration{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.job.ServiceAccountName = "kamaji"

			d := &Migrate{
				desiredDatastore: &kamajiv1alpha1.DataStore{
					Spec: kamajiv1alpha1.DataStoreSpec{Migration: tt.migration},
				},
				job: &batchv1.Job{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{Spec: tt.job},
					},
				},
			}

			d.setScheduling()

			got := d.job.Spec.Template.Spec
			if !reflect.DeepEqual(got.NodeSelector, tt.want.NodeSelector) {
				t.Errorf("nodeSelector = %v, want %v", got.NodeSelector, tt.want.NodeSelector)
			}
			if !reflect.DeepEqual(got.Tolerations, tt.want.Tolerations) {
				t.Errorf("tolerations = %v, want %v", got.Tolerations, tt.want.Tolerations)
			}
			if !reflect.DeepEqual(got.Affinity, tt.want.Affinity) {
				t.Errorf("affinity = %v, want %v", got.Affinity, tt.want.Affinity)
			}
			if got.ServiceAccountName != "kamaji" {
				t.Errorf("serviceAccountName = %q, want it untouched", got.ServiceAccountName)
			}
		})
	}
}
