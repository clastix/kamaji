// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/keyutil"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/utilities"
)

func generateServiceAccountKeyPair(t *testing.T) (publicKey, privateKey []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("cannot generate RSA key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("cannot marshal public key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func TestServiceAccountPublicKeyBundle(t *testing.T) {
	oldest, _ := generateServiceAccountKeyPair(t)
	previous, _ := generateServiceAccountKeyPair(t)
	current, currentKey := generateServiceAccountKeyPair(t)

	tests := []struct {
		name     string
		existing []byte
		want     []byte
	}{
		{name: "no existing key", existing: nil, want: current},
		{name: "invalid existing content", existing: []byte("not a PEM"), want: current},
		{name: "previous key is retained", existing: previous, want: append(bytes.Clone(current), previous...)},
		{name: "only the previous signing key is retained", existing: append(bytes.Clone(previous), oldest...), want: append(bytes.Clone(current), previous...)},
		{name: "same key is not duplicated", existing: current, want: current},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serviceAccountPublicKeyBundle(current, tt.existing)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("unexpected bundle:\n%s\nwant:\n%s", got, tt.want)
			}
			// The signing key must always be the first one, since it's used for the key pair validation.
			if valid, err := crypto.CheckPublicAndPrivateKeyValidity(got, currentKey); err != nil || !valid {
				t.Fatalf("bundle doesn't start with the signing key: valid=%t, err=%v", valid, err)
			}
			// kube-apiserver must be able to read every key from --service-account-key-file.
			keys, err := keyutil.ParsePublicKeysPEM(got)
			if err != nil {
				t.Fatalf("cannot parse bundle as kube-apiserver does: %v", err)
			}
			if wantKeys := bytes.Count(tt.want, []byte("-----BEGIN")); len(keys) != wantKeys {
				t.Fatalf("expected %d keys, got %d", wantKeys, len(keys))
			}
		})
	}
}

func TestSACertificatePrunePreviousKey(t *testing.T) {
	previous, _ := generateServiceAccountKeyPair(t)
	current, currentKey := generateServiceAccountKeyPair(t)

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}

	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "default", UID: "tcp-uid"},
	}

	r := &SACertificate{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	if err := r.Define(context.Background(), tcp); err != nil {
		t.Fatalf("cannot define resource: %v", err)
	}

	r.resource.UID = "secret-uid"
	r.resource.Annotations = map[string]string{utilities.PrunePreviousKeyRequestAnnotation: ""}
	r.resource.Data = map[string][]byte{
		kubeadmconstants.ServiceAccountPublicKeyName:  append(bytes.Clone(current), previous...),
		kubeadmconstants.ServiceAccountPrivateKeyName: currentKey,
	}

	if err := r.mutate(context.Background(), tcp)(); err != nil {
		t.Fatalf("mutate failed: %v", err)
	}

	if got := r.resource.Data[kubeadmconstants.ServiceAccountPublicKeyName]; !bytes.Equal(got, current) {
		t.Fatalf("expected only the signing public key, got:\n%s", got)
	}

	if !bytes.Equal(r.resource.Data[kubeadmconstants.ServiceAccountPrivateKeyName], currentKey) {
		t.Fatal("signing key must not change upon pruning")
	}

	if utilities.IsPreviousKeyPruneRequested(r.resource) {
		t.Fatal("prune annotation must be removed")
	}
}
