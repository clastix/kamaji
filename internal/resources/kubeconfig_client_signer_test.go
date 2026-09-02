// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	keyutil "k8s.io/client-go/util/keyutil"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	certstestutil "k8s.io/kubernetes/cmd/kubeadm/app/util/certs"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestGetClientSignerSecret(t *testing.T) {
	t.Parallel()

	clientCACert, clientCAKey := certstestutil.SetupCertificateAuthority(t)
	clientCAKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(clientCAKey)
	if err != nil {
		t.Fatalf("marshal client CA key: %v", err)
	}

	scheme := runtime.NewScheme()
	if err = corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	const (
		namespace          = "tenant"
		clientCASecret     = "client-ca"
		defaultCASecret    = "server-ca"
		tenantControlPlane = "tenant-control-plane"
	)
	dedicatedSigner := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clientCASecret, Namespace: namespace},
		Data: map[string][]byte{
			kubeadmconstants.CACertName: pkiutil.EncodeCertPEM(clientCACert),
			kubeadmconstants.CAKeyName:  clientCAKeyPEM,
		},
	}
	defaultSigner := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: defaultCASecret, Namespace: namespace},
	}
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane,
			Namespace: namespace,
			Annotations: map[string]string{
				kamajiv1alpha1.ClientCASecretAnnotation: clientCASecret,
			},
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dedicatedSigner).Build()

	got, err := GetClientSignerSecret(t.Context(), k8sClient, tcp, defaultSigner)
	if err != nil {
		t.Fatalf("get client signer: %v", err)
	}
	if got.Name != clientCASecret {
		t.Fatalf("expected signer %q, got %q", clientCASecret, got.Name)
	}
}

func TestGetClientSignerSecretDefaultsToServerCA(t *testing.T) {
	t.Parallel()

	defaultSigner := &corev1.Secret{}
	got, err := GetClientSignerSecret(
		t.Context(),
		fake.NewClientBuilder().Build(),
		&kamajiv1alpha1.TenantControlPlane{},
		defaultSigner,
	)
	if err != nil {
		t.Fatalf("get default client signer: %v", err)
	}
	if got != defaultSigner {
		t.Fatal("expected the server CA secret to remain the default signer")
	}
}
