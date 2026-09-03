// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"crypto/x509"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	certutil "k8s.io/client-go/util/cert"
	keyutil "k8s.io/client-go/util/keyutil"
	certstestutil "k8s.io/kubernetes/cmd/kubeadm/app/util/certs"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/clastix/kamaji/internal/utilities"
)

func TestGeneratedKubeconfigRejectsStaleClientIssuer(t *testing.T) {
	t.Parallel()

	const user = "test-user"
	groups := sets.New("test-group")
	oldClientCA, oldClientCAKey := certstestutil.SetupCertificateAuthority(t)
	newClientCA, _ := certstestutil.SetupCertificateAuthority(t)
	clientCertificate, clientKey, err := pkiutil.NewCertAndKey(
		oldClientCA,
		oldClientCAKey,
		&pkiutil.CertConfig{
			Config: certutil.Config{
				CommonName:   user,
				Organization: groups.UnsortedList(),
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			NotAfter: time.Now().Add(time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("create client certificate: %v", err)
	}
	clientKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(clientKey)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}

	serverCA, _ := certstestutil.SetupCertificateAuthority(t)
	template := kubeconfigForCertificate(nil, nil, pkiutil.EncodeCertPEM(serverCA), user)
	concrete := kubeconfigForCertificate(
		pkiutil.EncodeCertPEM(clientCertificate),
		clientKeyPEM,
		pkiutil.EncodeCertPEM(serverCA),
		user,
	)
	concreteBytes, err := utilities.EncodeToYaml(concrete)
	if err != nil {
		t.Fatalf("encode kubeconfig: %v", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "generated-kubeconfig"},
		Data:       map[string][]byte{"value": concreteBytes},
	}

	reconciler := &KubeconfigGeneratorReconciler{}
	valid, err := reconciler.isValid(
		secret,
		template,
		groups,
		user,
		pkiutil.EncodeCertPEM(newClientCA),
	)
	if err != nil {
		t.Fatalf("validate generated kubeconfig: %v", err)
	}
	if valid {
		t.Fatal("expected a kubeconfig signed by the stale client CA to be invalid")
	}
}

func kubeconfigForCertificate(clientCertificate, clientKey, serverCA []byte, user string) *clientcmdapiv1.Config {
	const cluster = "tenant"

	return &clientcmdapiv1.Config{
		Kind:       "Config",
		APIVersion: "v1",
		AuthInfos: []clientcmdapiv1.NamedAuthInfo{
			{
				Name: user,
				AuthInfo: clientcmdapiv1.AuthInfo{
					ClientCertificateData: clientCertificate,
					ClientKeyData:         clientKey,
				},
			},
		},
		Clusters: []clientcmdapiv1.NamedCluster{
			{
				Name: cluster,
				Cluster: clientcmdapiv1.Cluster{
					Server:                   "https://tenant.example.com:6443",
					CertificateAuthorityData: serverCA,
				},
			},
		},
		Contexts: []clientcmdapiv1.NamedContext{
			{
				Name: user,
				Context: clientcmdapiv1.Context{
					Cluster:  cluster,
					AuthInfo: user,
				},
			},
		},
		CurrentContext: user,
	}
}
