// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

func GetCertificate(cert []byte) (*x509.Certificate, error) {
	pemContent, _ := pem.Decode(cert)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	return x509.ParseCertificate(pemContent.Bytes)
}

func GetPrivateKey(privKey []byte) (*rsa.PrivateKey, error) {
	pemContent, _ := pem.Decode(privKey)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	return x509.ParsePKCS1PrivateKey(pemContent.Bytes)
}

func GetPublickKey(pubKey []byte) (*rsa.PublicKey, error) {
	pemContent, _ := pem.Decode(pubKey)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(pemContent.Bytes)
	if err != nil {
		return nil, err
	}

	return pub.(*rsa.PublicKey), nil // nolint:forcetypeassert
}

func GenerateCertificateKeyPairBytes(template *x509.Certificate, bitSize int, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*bytes.Buffer, *bytes.Buffer, error) {
	certPrivKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &certPrivKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := &bytes.Buffer{}
	if err := pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return nil, nil, err
	}

	certPrivKeyPEM := &bytes.Buffer{}
	if err := pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	}); err != nil {
		return nil, nil, err
	}

	return certPEM, certPrivKeyPEM, nil
}

func IsValidKeyPairBytes(pubKeyBytes []byte, privKeyBytes []byte) (bool, error) {
	privKey, err := GetPrivateKey(privKeyBytes)
	if err != nil {
		return false, err
	}

	pubKey, err := GetPublickKey(pubKeyBytes)
	if err != nil {
		return false, err
	}

	return checkPublicKeys(privKey.PublicKey, *pubKey), nil
}

func IsValidCertificateKeyPairBytes(certBytes []byte, privKeyBytes []byte) (bool, error) {
	cert, err := GetCertificate(certBytes)
	if err != nil {
		return false, err
	}

	privKey, err := GetPrivateKey(privKeyBytes)
	if err != nil {
		return false, err
	}

	return isValidCertificateKeyPairBytes(*cert, *privKey), nil
}

func isValidCertificateKeyPairBytes(cert x509.Certificate, privKey rsa.PrivateKey) bool {
	return checkCertificateValidity(cert) && checkCertificateKeyPair(cert, privKey)
}

func checkCertificateValidity(cert x509.Certificate) bool {
	now := time.Now()

	return now.Before(cert.NotAfter) && now.After(cert.NotBefore)
}

func checkCertificateKeyPair(cert x509.Certificate, privKey rsa.PrivateKey) bool {
	return checkPublicKeys(*cert.PublicKey.(*rsa.PublicKey), privKey.PublicKey) // nolint:forcetypeassert
}

func checkPublicKeys(a rsa.PublicKey, b rsa.PublicKey) bool {
	isN := a.N.Cmp(b.N) == 0
	isE := a.E == b.E

	return isN && isE
}
