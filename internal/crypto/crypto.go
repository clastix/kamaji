// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"net"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

// CheckPublicAndPrivateKeyValidity checks if the given bytes for the private and public keys are valid.
func CheckPublicAndPrivateKeyValidity(publicKey, privateKey []byte) (bool, error) {
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

	return checkPublicKeys(pubKey, privKey), nil
}

// CheckCertificateNamesAndIPs checks if the Kubernetes API Server certificate matches the Control Plane Endpoint and SAN stored in the kubeadm:
// it must check both IPs and DNS names, and returns a false if the required entry isn't available.
// In case of removal of entries, this function returns true nevertheless to avoid reloading a Control Plane uselessly.
func CheckCertificateNamesAndIPs(certificateBytes []byte, entries []string) (bool, error) {
	crt, err := ParseCertificateBytes(certificateBytes)
	if err != nil {
		return false, err
	}

	ips := sets.New[string]()
	for _, ip := range crt.IPAddresses {
		ips.Insert(ip.String())
	}

	dns := sets.New[string](crt.DNSNames...)

	for _, e := range entries {
		if ip := net.ParseIP(e); ip != nil {
			if !ips.Has(ip.String()) {
				return false, nil
			}

			continue
		}

		if !dns.Has(e) {
			return false, nil
		}
	}

	return true, nil
}

// CheckCertificateAndPrivateKeyPairValidity checks if the certificate and private key pair are valid.
func CheckCertificateAndPrivateKeyPairValidity(certificate, privateKey []byte, threshold time.Duration) (bool, error) {
	switch {
	case len(certificate) == 0, len(privateKey) == 0:
		return false, nil
	default:
		return IsValidCertificateKeyPairBytes(certificate, privateKey, threshold)
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
		return nil, nil, fmt.Errorf("provided CA private key for certificate generation cannot be parsed: %w", err)
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
		return nil, fmt.Errorf("cannot parse x509 Certificate: %w", err)
	}

	return crt, nil
}

// ParsePrivateKeyBytes takes the private key bytes returning an RSA private key by parsing it.
func ParsePrivateKeyBytes(content []byte) (crypto.Signer, error) {
	pemContent, _ := pem.Decode(content)
	if pemContent == nil {
		return nil, fmt.Errorf("no right PEM block")
	}

	if pemContent.Type == "EC PRIVATE KEY" {
		privateKey, err := x509.ParseECPrivateKey(pemContent.Bytes)
		if err != nil {
			return nil, fmt.Errorf("cannot parse EC Private Key: %w", err)
		}

		return privateKey, nil
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(pemContent.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse PKCS1 Private Key: %w", err)
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
func IsValidCertificateKeyPairBytes(certificateBytes, privateKeyBytes []byte, expirationThreshold time.Duration) (bool, error) {
	crt, err := ParseCertificateBytes(certificateBytes)
	if err != nil {
		return false, err
	}

	key, err := ParsePrivateKeyBytes(privateKeyBytes)
	if err != nil {
		return false, err
	}

	switch {
	case !checkCertificateValidity(*crt, expirationThreshold):
		return false, nil
	case !checkPublicKeys(crt.PublicKey, key):
		return false, nil
	default:
		return true, nil
	}
}

func VerifyCertificate(cert, ca []byte, usages ...x509.ExtKeyUsage) (bool, error) {
	if len(usages) == 0 {
		return false, fmt.Errorf("missing usages for certificate verification")
	}

	crt, err := ParseCertificateBytes(cert)
	if err != nil {
		return false, err
	}

	caCrt, err := ParseCertificateBytes(ca)
	if err != nil {
		return false, err
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCrt)

	chains, err := crt.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: usages,
	})

	return len(chains) > 0, err
}

func generateCertificateKeyPairBytes(template *x509.Certificate, caCert *x509.Certificate, caKey crypto.Signer) (*bytes.Buffer, *bytes.Buffer, error) {
	certPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot generate an RSA key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(cryptorand.Reader, template, caCert, &certPrivKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create the certificate: %w", err)
	}

	certPEM := &bytes.Buffer{}
	if err = pem.Encode(certPEM, &pem.Block{
		Type:    "CERTIFICATE",
		Headers: nil,
		Bytes:   certBytes,
	}); err != nil {
		return nil, nil, fmt.Errorf("cannot encode the generate certificate bytes: %w", err)
	}

	certPrivKeyPEM := &bytes.Buffer{}
	if err = pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(certPrivKey),
	}); err != nil {
		return nil, nil, fmt.Errorf("cannot encode private key: %w", err)
	}

	return certPEM, certPrivKeyPEM, nil
}

func checkCertificateValidity(cert x509.Certificate, threshold time.Duration) bool {
	// Avoiding waiting for the exact expiration date by creating a one-day gap
	notAfter := cert.NotAfter.After(time.Now().Add(threshold))
	notBefore := cert.NotBefore.Before(time.Now())

	return notAfter && notBefore
}

func checkPublicKeys(a crypto.PublicKey, b crypto.Signer) bool {
	if key, ok := a.(interface{ Equal(k crypto.PublicKey) bool }); ok {
		return key.Equal(b.Public())
	}

	return false
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
