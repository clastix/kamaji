// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"bytes"
	"crypto/x509"
	"testing"

	keyutil "k8s.io/client-go/util/keyutil"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	certstestutil "k8s.io/kubernetes/cmd/kubeadm/app/util/certs"
	kubeadmconfig "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	kamajicrypto "github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/utilities"
)

func TestCreateKubeconfigWithClientSigner(t *testing.T) {
	serverCA, _ := certstestutil.SetupCertificateAuthority(t)
	clientCA, clientCAKey := certstestutil.SetupCertificateAuthority(t)

	clientCAKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(clientCAKey)
	if err != nil {
		t.Fatalf("marshal client CA key: %v", err)
	}

	config, err := kubeadmconfig.DefaultedStaticInitConfiguration()
	if err != nil {
		t.Fatalf("create kubeadm configuration: %v", err)
	}

	config.CertificatesDir = t.TempDir()
	config.ClusterName = "tenant"
	config.ControlPlaneEndpoint = "127.0.0.1:6443"

	serverCAPEM := pkiutil.EncodeCertPEM(serverCA)
	clientCAPEM := pkiutil.EncodeCertPEM(clientCA)
	kubeconfig, err := CreateKubeconfigWithClientSigner(
		kubeadmconstants.AdminKubeConfigFileName,
		serverCAPEM,
		CertificatePrivateKeyPair{Certificate: clientCAPEM, PrivateKey: clientCAKeyPEM},
		&Configuration{InitConfiguration: *config},
	)
	if err != nil {
		t.Fatalf("create kubeconfig: %v", err)
	}

	decoded, err := utilities.DecodeKubeconfigYAML(kubeconfig)
	if err != nil {
		t.Fatalf("decode kubeconfig: %v", err)
	}
	if len(decoded.Clusters) != 1 || len(decoded.AuthInfos) != 1 {
		t.Fatalf("unexpected kubeconfig shape: %d clusters, %d auth infos", len(decoded.Clusters), len(decoded.AuthInfos))
	}
	if !bytes.Equal(decoded.Clusters[0].Cluster.CertificateAuthorityData, serverCAPEM) {
		t.Fatal("kubeconfig does not trust the server CA")
	}

	clientCertificate := decoded.AuthInfos[0].AuthInfo.ClientCertificateData
	if valid, verifyErr := kamajicrypto.VerifyCertificate(clientCertificate, clientCAPEM, x509.ExtKeyUsageClientAuth); !valid || verifyErr != nil {
		t.Fatalf("client certificate is not signed by client CA: %v", verifyErr)
	}
	if valid, _ := kamajicrypto.VerifyCertificate(clientCertificate, serverCAPEM, x509.ExtKeyUsageClientAuth); valid {
		t.Fatal("client certificate is unexpectedly signed by server CA")
	}
}

func TestIsKubeconfigClientCertificateValid(t *testing.T) {
	t.Parallel()

	serverCA, _ := certstestutil.SetupCertificateAuthority(t)
	oldClientCA, oldClientCAKey := certstestutil.SetupCertificateAuthority(t)
	newClientCA, _ := certstestutil.SetupCertificateAuthority(t)

	oldClientCAKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(oldClientCAKey)
	if err != nil {
		t.Fatalf("marshal old client CA key: %v", err)
	}

	for _, kubeconfigName := range []string{
		kubeadmconstants.AdminKubeConfigFileName,
		kubeadmconstants.SuperAdminKubeConfigFileName,
	} {
		t.Run(kubeconfigName, func(t *testing.T) {
			t.Parallel()

			config, configErr := kubeadmconfig.DefaultedStaticInitConfiguration()
			if configErr != nil {
				t.Fatalf("create kubeadm configuration: %v", configErr)
			}

			config.CertificatesDir = t.TempDir()
			config.ClusterName = "tenant"
			config.ControlPlaneEndpoint = "127.0.0.1:6443"

			oldClientCAPEM := pkiutil.EncodeCertPEM(oldClientCA)
			kubeconfig, createErr := CreateKubeconfigWithClientSigner(
				kubeconfigName,
				pkiutil.EncodeCertPEM(serverCA),
				CertificatePrivateKeyPair{Certificate: oldClientCAPEM, PrivateKey: oldClientCAKeyPEM},
				&Configuration{InitConfiguration: *config},
			)
			if createErr != nil {
				t.Fatalf("create kubeconfig: %v", createErr)
			}

			if !IsKubeconfigClientCertificateValid(kubeconfig, oldClientCAPEM) {
				t.Fatal("expected the current client signer to validate the kubeconfig")
			}
			if IsKubeconfigClientCertificateValid(kubeconfig, pkiutil.EncodeCertPEM(newClientCA)) {
				t.Fatal("expected a stale client signer to invalidate the kubeconfig")
			}
		})
	}
}
