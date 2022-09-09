// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"

	cryptoKamaji "github.com/clastix/kamaji/internal/crypto"
)

func GenerateCACertificatePrivateKeyPair(baseName string, config *Configuration) (*CertificatePrivateKeyPair, error) {
	defer deleteCertificateDirectory(config.InitConfiguration.CertificatesDir)

	kubeadmCert, err := getKubeadmCert(baseName)
	if err != nil {
		return nil, err
	}

	if _, _, err = initPhaseAsCA(kubeadmCert, config); err != nil {
		return nil, err
	}

	contents, err := readCertificateFiles(baseName, config.InitConfiguration.CertificatesDir, "crt", "key")
	if err != nil {
		return nil, err
	}

	certificatePrivateKeyPair := &CertificatePrivateKeyPair{
		Certificate: contents[0],
		PrivateKey:  contents[1],
	}

	return certificatePrivateKeyPair, err
}

func GenerateCertificatePrivateKeyPair(baseName string, config *Configuration, ca CertificatePrivateKeyPair) (*CertificatePrivateKeyPair, error) {
	defer deleteCertificateDirectory(config.InitConfiguration.CertificatesDir)

	certificate, _ := cryptoKamaji.ParseCertificateBytes(ca.Certificate)
	signer, _ := cryptoKamaji.ParsePrivateKeyBytes(ca.PrivateKey)

	kubeadmCert, err := getKubeadmCert(baseName)
	if err != nil {
		return nil, err
	}

	if err = initPhaseFromCA(kubeadmCert, config, certificate, signer); err != nil {
		return nil, err
	}

	contents, err := readCertificateFiles(baseName, config.InitConfiguration.CertificatesDir, "crt", "key")
	if err != nil {
		return nil, err
	}

	certificatePrivateKeyPair := &CertificatePrivateKeyPair{
		Certificate: contents[0],
		PrivateKey:  contents[1],
	}

	return certificatePrivateKeyPair, err
}

func getKubeadmCert(baseName string) (*certs.KubeadmCert, error) {
	switch baseName {
	case kubeadmconstants.CACertAndKeyBaseName:
		return certs.KubeadmCertRootCA(), nil
	case kubeadmconstants.APIServerCertAndKeyBaseName:
		return certs.KubeadmCertAPIServer(), nil
	case kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName:
		return certs.KubeadmCertKubeletClient(), nil
	case kubeadmconstants.FrontProxyCACertAndKeyBaseName:
		return certs.KubeadmCertFrontProxyCA(), nil
	case kubeadmconstants.FrontProxyClientCertAndKeyBaseName:
		return certs.KubeadmCertFrontProxyClient(), nil
	default:
		return nil, fmt.Errorf("wrong ca file name %s", baseName)
	}
}

func GeneratePublicKeyPrivateKeyPair(baseName string, config *Configuration) (*PublicKeyPrivateKeyPair, error) {
	defer deleteCertificateDirectory(config.InitConfiguration.CertificatesDir)

	if err := initPhaseCertsSA(config); err != nil {
		return nil, err
	}

	contents, err := readCertificateFiles(baseName, config.InitConfiguration.CertificatesDir, "pub", "key")
	if err != nil {
		return nil, err
	}

	publicKeyPrivateKeyPair := &PublicKeyPrivateKeyPair{
		PublicKey:  contents[0],
		PrivateKey: contents[1],
	}

	return publicKeyPrivateKeyPair, err
}

func initPhaseCertsSA(config *Configuration) error {
	return certs.CreateServiceAccountKeyAndPublicKeyFiles(config.InitConfiguration.CertificatesDir, config.InitConfiguration.PublicKeyAlgorithm())
}

func initPhaseFromCA(kubeadmCert *certs.KubeadmCert, config *Configuration, certificate *x509.Certificate, signer crypto.Signer) error {
	return kubeadmCert.CreateFromCA(&config.InitConfiguration, certificate, signer)
}

func initPhaseAsCA(kubeadmCert *certs.KubeadmCert, config *Configuration) (*x509.Certificate, crypto.Signer, error) {
	return kubeadmCert.CreateAsCA(&config.InitConfiguration)
}

func readCertificateFiles(name string, directory string, extensions ...string) ([][]byte, error) {
	result := make([][]byte, 0, len(extensions))

	for _, extension := range extensions {
		fileName := fmt.Sprintf("%s.%s", name, extension)
		path := filepath.Join(directory, fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		result = append(result, content)
	}

	return result, nil
}

func deleteCertificateDirectory(certificateDirectory string) {
	if err := os.RemoveAll(certificateDirectory); err != nil {
		// TODO(prometherion): we should log rather than printing to stdout
		fmt.Printf("Error removing %s: %s", certificateDirectory, err.Error()) //nolint:forbidigo
	}
}
