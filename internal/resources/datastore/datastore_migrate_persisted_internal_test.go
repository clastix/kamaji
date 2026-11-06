// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestMigrateJobStableOncePersisted(t *testing.T) {
	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("unable to start envtest: %v", err)
	}

	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("unable to stop envtest: %v", err)
		}
	})

	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("unable to create client: %v", err)
	}

	tests := []struct {
		name      string
		migration *kamajiv1alpha1.DataStoreMigration
	}{
		{
			name: "no migration settings",
		},
		{
			name:      "scheduling settings",
			migration: newSchedulingMigration(),
		},
		{
			name: "toleration without operator",
			migration: &kamajiv1alpha1.DataStoreMigration{
				Tolerations: []corev1.Toleration{
					{Key: "dedicated", Value: "migration", Effect: corev1.TaintEffectNoSchedule},
					{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists},
				},
			},
		},
		{
			name: "security contexts",
			migration: &kamajiv1alpha1.DataStoreMigration{
				PodSecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot:   ptr.To(true),
					RunAsUser:      ptr.To[int64](1000),
					SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					ReadOnlyRootFilesystem:   ptr.To(true),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			d := &Migrate{
				desiredDatastore: &kamajiv1alpha1.DataStore{
					Spec: kamajiv1alpha1.DataStoreSpec{Migration: tt.migration},
				},
				job: &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migrate-" + string(rune('a'+i)),
						Namespace: "default",
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers:    []corev1.Container{{Name: "migrate", Image: "migrate"}},
							},
						},
					},
				},
			}

			d.setScheduling()
			d.setSecurityContext()

			if err := c.Create(ctx, d.job); err != nil {
				t.Fatalf("unable to create Job: %v", err)
			}

			stored := &batchv1.Job{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(d.job), stored); err != nil {
				t.Fatalf("unable to get Job: %v", err)
			}

			d.job = stored.DeepCopy()
			d.setScheduling()
			d.setSecurityContext()

			if !equality.Semantic.DeepEqual(stored.Spec.Template.Spec, d.job.Spec.Template.Spec) {
				t.Errorf("reconciling the stored Job changes its template:\nstored: %+v\nwanted: %+v", stored.Spec.Template.Spec, d.job.Spec.Template.Spec)
			}
		})
	}
}
