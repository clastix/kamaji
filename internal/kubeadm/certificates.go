// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

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

	certificate, err := cryptoKamaji.ParseCertificateBytes(ca.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	signer, err := cryptoKamaji.ParsePrivateKeyBytes(ca.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	kubeadmCert, err := getKubeadmCert(baseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeadm cert: %w", err)
	}

	if err = initPhaseFromCA(kubeadmCert, config, certificate, signer); err != nil {
		return nil, fmt.Errorf("failed to initialize phase from CA: %w", err)
	}

	contents, err := readCertificateFiles(baseName, config.InitConfiguration.CertificatesDir, "crt", "key")
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate files: %w", err)
	}

	if len(contents) != 2 {
		return nil, fmt.Errorf("unexpected number of certificate files: expected 2, got %d", len(contents))
	}

	certificatePrivateKeyPair := &CertificatePrivateKeyPair{
		Certificate: contents[0],
		PrivateKey:  contents[1],
	}

	return certificatePrivateKeyPair, nil
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
	return certs.CreateServiceAccountKeyAndPublicKeyFiles(config.InitConfiguration.CertificatesDir, config.InitConfiguration.EncryptionAlgorithm)
}

func initPhaseFromCA(kubeadmCert *certs.KubeadmCert, config *Configuration, certificate *x509.Certificate, signer crypto.Signer) error {
	certConfig, err := certificateConfig(kubeadmCert, config, time.Now())
	if err != nil {
		return err
	}

	cert, key, err := pkiutil.NewCertAndKey(certificate, signer, certConfig)
	if err != nil {
		return fmt.Errorf("failed to generate %q certificate: %w", kubeadmCert.Name, err)
	}

	return pkiutil.WriteCertAndKey(config.InitConfiguration.CertificatesDir, kubeadmCert.BaseName, cert, key)
}

func initPhaseAsCA(kubeadmCert *certs.KubeadmCert, config *Configuration) (*x509.Certificate, crypto.Signer, error) {
	certConfig, err := certificateConfig(kubeadmCert, config, time.Now())
	if err != nil {
		return nil, nil, err
	}

	cert, key, err := pkiutil.NewCertificateAuthority(certConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate %q CA certificate: %w", kubeadmCert.Name, err)
	}

	if err = pkiutil.WriteCertAndKey(config.InitConfiguration.CertificatesDir, kubeadmCert.BaseName, cert, key); err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

// certificateConfig returns the kubeadm configuration of the certificate, valid from issuedAt.
// kubeadm dates certificates from the start time of its process, which is fine for the CLI,
// but a long-running controller would issue certificates dated from when it started:
// the validity window is shifted to issuedAt, keeping its length.
func certificateConfig(kubeadmCert *certs.KubeadmCert, config *Configuration, issuedAt time.Time) (*pkiutil.CertConfig, error) {
	certConfig, err := kubeadmCert.GetConfig(&config.InitConfiguration)
	if err != nil {
		return nil, fmt.Errorf("failed to get %q certificate configuration: %w", kubeadmCert.Name, err)
	}

	shift := issuedAt.UTC().Add(-kubeadmconstants.CertificateBackdate).Sub(certConfig.NotBefore)

	certConfig.NotBefore = certConfig.NotBefore.Add(shift)
	if !certConfig.NotAfter.IsZero() {
		certConfig.NotAfter = certConfig.NotAfter.Add(shift)
	}

	return certConfig, nil
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
