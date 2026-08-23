// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestGetRESTClientConfigEmptyKubeconfig(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-kubeconfig", Namespace: "default"},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: []byte("apiVersion: v1\nkind: Config\n"),
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "default"},
	}
	tcp.Status.KubeConfig.Admin.SecretName = secret.Name

	if _, err := GetRESTClientConfig(context.Background(), c, tcp); err == nil {
		t.Fatal("expected an error for a kubeconfig without cluster or user entries, got nil")
	}
}
