// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

const testKubeconfigSecretName = "admin-kubeconfig"

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}

	return scheme
}

func testKubeconfigYAML(server string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %q
    certificate-authority-data: %s
  name: tenant
contexts:
- context:
    cluster: tenant
    user: kubernetes-super-admin
  name: kubernetes-super-admin@tenant
current-context: kubernetes-super-admin@tenant
users:
- name: kubernetes-super-admin
  user:
    client-certificate-data: %s
    client-key-data: %s
`,
		server,
		base64.StdEncoding.EncodeToString([]byte("fake-ca")),
		base64.StdEncoding.EncodeToString([]byte("fake-certificate")),
		base64.StdEncoding.EncodeToString([]byte("fake-key")),
	))
}

func testTenantControlPlane(t *testing.T, secret *corev1.Secret, annotations map[string]string) *kamajiv1alpha1.TenantControlPlane {
	t.Helper()

	port := int32(6443)
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "tcp",
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				Port: port,
			},
		},
	}
	tcp.Status.KubeConfig.Admin.SecretName = secret.Name

	return tcp
}

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

func TestGetRESTClientConfigServiceHostWithoutAnnotation(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testKubeconfigSecretName, Namespace: "default"},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: testKubeconfigYAML("https://100.64.0.22:6443"),
		},
	}

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(secret).Build()

	config, err := GetRESTClientConfig(context.Background(), c, testTenantControlPlane(t, secret, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expected := "https://tcp.default.svc:6443"; config.Host != expected {
		t.Errorf("expected Host %q, got %q", expected, config.Host)
	}
}

func TestGetRESTClientConfigKubeconfigServerWithAnnotation(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testKubeconfigSecretName, Namespace: "default"},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: testKubeconfigYAML("https://100.64.0.22:6443"),
		},
	}

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(secret).Build()

	tcp := testTenantControlPlane(t, secret, map[string]string{
		kamajiv1alpha1.KubeconfigSecretKeyAnnotation: kubeadmconstants.SuperAdminKubeConfigFileName,
	})

	config, err := GetRESTClientConfig(context.Background(), c, tcp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expected := "https://100.64.0.22:6443"; config.Host != expected {
		t.Errorf("expected Host %q, got %q", expected, config.Host)
	}
}

func TestGetRESTClientConfigKubeconfigServerWithCustomKeyAnnotation(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testKubeconfigSecretName, Namespace: "default"},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: testKubeconfigYAML("https://100.64.0.22:6443"),
			kubeadmconstants.AdminKubeConfigFileName:      testKubeconfigYAML("https://100.64.0.23:6443"),
		},
	}

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(secret).Build()

	tcp := testTenantControlPlane(t, secret, map[string]string{
		kamajiv1alpha1.KubeconfigSecretKeyAnnotation: kubeadmconstants.AdminKubeConfigFileName,
	})

	config, err := GetRESTClientConfig(context.Background(), c, tcp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expected := "https://100.64.0.23:6443"; config.Host != expected {
		t.Errorf("expected Host %q, got %q", expected, config.Host)
	}
}

func TestGetRESTClientConfigServiceHostWhenKubeconfigServerIsEmpty(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testKubeconfigSecretName, Namespace: "default"},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: testKubeconfigYAML(""),
		},
	}

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(secret).Build()

	tcp := testTenantControlPlane(t, secret, map[string]string{
		kamajiv1alpha1.KubeconfigSecretKeyAnnotation: kubeadmconstants.SuperAdminKubeConfigFileName,
	})

	config, err := GetRESTClientConfig(context.Background(), c, tcp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expected := "https://tcp.default.svc:6443"; config.Host != expected {
		t.Errorf("expected Host %q, got %q", expected, config.Host)
	}
}

func TestGetRESTClientConfigMissingSecretKeyWithAnnotation(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testKubeconfigSecretName, Namespace: "default"},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: testKubeconfigYAML("https://100.64.0.22:6443"),
		},
	}

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(secret).Build()

	tcp := testTenantControlPlane(t, secret, map[string]string{
		kamajiv1alpha1.KubeconfigSecretKeyAnnotation: "unknown-key",
	})

	if _, err := GetRESTClientConfig(context.Background(), c, tcp); err == nil {
		t.Fatal("expected an error for a missing kubeconfig secret key, got nil")
	}
}
