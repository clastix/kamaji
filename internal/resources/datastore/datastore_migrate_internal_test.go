// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestMutateJobSecurityContext(t *testing.T) {
	d := &Migrate{
		MigrateImage: "kamaji:edge",
		job:          &batchv1.Job{},
	}

	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-tcp", Namespace: "default"},
		Spec:       kamajiv1alpha1.TenantControlPlaneSpec{DataStore: "kamaji-etcd"},
	}

	if err := d.mutateJob(tcp); err != nil {
		t.Fatalf("mutateJob() returned an error: %v", err)
	}

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

	// No runAsUser is enforced: the image's own non-root user applies.
	if got := d.job.Spec.Template.Spec.SecurityContext.RunAsUser; got != nil {
		t.Errorf("pod runAsUser = %v, want nil", *got)
	}
}
