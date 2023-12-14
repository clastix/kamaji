// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"os"
	"path"
	"path/filepath"

	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"

	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/utilities"
)

func buildCertificateDirectoryWithCA(ca CertificatePrivateKeyPair, directory string) error {
	if err := os.MkdirAll(directory, os.FileMode(0o755)); err != nil {
		return err
	}

	certPath := path.Join(directory, kubeadmconstants.CACertName)
	if err := os.WriteFile(certPath, ca.Certificate, os.FileMode(0o600)); err != nil {
		return err
	}

	keyPath := path.Join(directory, kubeadmconstants.CAKeyName)

	return os.WriteFile(keyPath, ca.PrivateKey, os.FileMode(0o600))
}

func CreateKubeconfig(kubeconfigName string, ca CertificatePrivateKeyPair, config *Configuration) ([]byte, error) {
	if err := buildCertificateDirectoryWithCA(ca, config.InitConfiguration.CertificatesDir); err != nil {
		return nil, err
	}

	defer deleteCertificateDirectory(config.InitConfiguration.CertificatesDir)

	if err := kubeconfig.CreateKubeConfigFile(kubeconfigName, config.InitConfiguration.CertificatesDir, &config.InitConfiguration); err != nil {
		return nil, err
	}

	path := filepath.Join(config.InitConfiguration.CertificatesDir, kubeconfigName)

	return os.ReadFile(path)
}

func IsKubeconfigValid(bytes []byte) bool {
	kc, err := utilities.DecodeKubeconfigYAML(bytes)
	if err != nil {
		return false
	}

	ok, _ := crypto.IsValidCertificateKeyPairBytes(kc.AuthInfos[0].AuthInfo.ClientCertificateData, kc.AuthInfos[0].AuthInfo.ClientKeyData)

	return ok
}
