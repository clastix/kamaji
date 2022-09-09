// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"time"

	"github.com/pkg/errors"
)

// CheckPublicAndPrivateKeyValidity checks if the given bytes for the private and public keys are valid.
func CheckPublicAndPrivateKeyValidity(publicKey []byte, privateKey []byte) (bool, error) {
	if len(publicKey) == 0 || len(privateKey) == 0 {
		return false, nil
	}

	pubKey, err := ParsePublicKeyBytes(publicKey)
	if err != nil {
		return false, err
	}

	privKey, err := ParsePrivateKeyBytes(privateKey)
	if err != nil {
		return false, err
	}

	return checkPublicKeys(privKey.PublicKey, *pubKey), nil
}

// CheckCertificateAndPrivateKeyPairValidity checks if the certificate and private key pair are valid.
func CheckCertificateAndPrivateKeyPairValidity(certificate []byte, privateKey []byte) (bool, error) {
	switch {
	case len(certificate) == 0, len(privateKey) == 0:
		return false, nil
	default:
		return IsValidCertificateKeyPairBytes(certificate, privateKey)
	}
}

// GenerateCertificatePrivateKeyPair starts from the Certificate Authority bytes a certificate using the provided
// template, returning the bytes both for the certificate and its key.
func GenerateCertificatePrivateKeyPair(template *x509.Certificate, caCertificate []byte, caPrivateKey []byte) (*bytes.Buffer, *bytes.Buffer, error) {
	caCertBytes, err := ParseCertificateBytes(caCertificate)
	if err != nil {
		return nil, nil, err
	}

	caPrivKeyBytes, err := ParsePrivateKeyBytes(caPrivateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "provided CA private key for certificate generation cannot be parsed")
	}

	return generateCertificateKeyPairBytes(template, caCertBytes, caPrivKeyBytes)
}

// ParseCertificateBytes takes the certificate bytes returning a x509 certificate by parsing it.
func ParseCertificateBytes(content []byte) (*x509.Certificate, error) {
	pemContent, _ := pem.Decode(content)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	crt, err := x509.ParseCertificate(pemContent.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse x509 Certificate")
	}

	return crt, nil
}

// ParsePrivateKeyBytes takes the private key bytes returning an RSA private key by parsing it.
func ParsePrivateKeyBytes(content []byte) (*rsa.PrivateKey, error) {
	pemContent, _ := pem.Decode(content)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(pemContent.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse PKCS1 Private Key")
	}

	return privateKey, nil
}

// ParsePublicKeyBytes takes the public key bytes returning an RSA public key by parsing it.
func ParsePublicKeyBytes(content []byte) (*rsa.PublicKey, error) {
	pemContent, _ := pem.Decode(content)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	publicKey, err := x509.ParsePKIXPublicKey(pemContent.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected *rsa.PublicKey, got %T", rsaPublicKey)
	}

	return rsaPublicKey, nil
}

// IsValidCertificateKeyPairBytes checks if the certificate matches the private key bounded to it.
func IsValidCertificateKeyPairBytes(certificateBytes []byte, privateKeyBytes []byte) (bool, error) {
	crt, err := ParseCertificateBytes(certificateBytes)
	if err != nil {
		return false, err
	}

	key, err := ParsePrivateKeyBytes(privateKeyBytes)
	if err != nil {
		return false, err
	}

	switch {
	case !checkCertificateValidity(*crt):
		return false, nil
	case !checkPublicKeys(*crt.PublicKey.(*rsa.PublicKey), key.PublicKey): //nolint:forcetypeassert
		return false, nil
	default:
		return true, nil
	}
}

func generateCertificateKeyPairBytes(template *x509.Certificate, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*bytes.Buffer, *bytes.Buffer, error) {
	certPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot generate an RSA key")
	}

	certBytes, err := x509.CreateCertificate(cryptorand.Reader, template, caCert, &certPrivKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create the certificate")
	}

	certPEM := &bytes.Buffer{}
	if err = pem.Encode(certPEM, &pem.Block{
		Type:    "CERTIFICATE",
		Headers: nil,
		Bytes:   certBytes,
	}); err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode the generate certificate bytes")
	}

	certPrivKeyPEM := &bytes.Buffer{}
	if err = pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(certPrivKey),
	}); err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode private key")
	}

	return certPEM, certPrivKeyPEM, nil
}

func checkCertificateValidity(cert x509.Certificate) bool {
	now := time.Now()

	return now.Before(cert.NotAfter) && now.After(cert.NotBefore)
}

func checkPublicKeys(a rsa.PublicKey, b rsa.PublicKey) bool {
	isN := a.N.Cmp(b.N) == 0
	isE := a.E == b.E

	return isN && isE
}

// NewCertificateTemplate returns the template that must be used to generate a certificate,
// used to perform the authentication against the DataStore.
func NewCertificateTemplate(commonName string) *x509.Certificate {
	return &x509.Certificate{
		PublicKeyAlgorithm: x509.RSA,
		SerialNumber:       big.NewInt(mathrand.Int63()),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"system:masters"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageCodeSigning,
		},
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
}
