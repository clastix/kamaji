// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"math/rand"
	"time"

	"github.com/clastix/kamaji/internal/crypto"
)

func GetETCDCACertificateAndKeyPair(tenant string, caCert []byte, caPrivKey []byte) (*bytes.Buffer, *bytes.Buffer, error) {
	template := getCertTemplate(tenant)

	return crypto.GetCertificateAndKeyPair(template, caCert, caPrivKey)
}

func IsETCDCertificateAndKeyPairValid(cert []byte, privKey []byte) (bool, error) {
	return crypto.IsValidCertificateKeyPairBytes(cert, privKey)
}

func getCertTemplate(tenant string) *x509.Certificate {
	serialNumber := big.NewInt(rand.Int63())

	return &x509.Certificate{
		PublicKeyAlgorithm: x509.RSA,
		SerialNumber:       serialNumber,
		Subject: pkix.Name{
			CommonName:   tenant,
			Organization: []string{certOrganization},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(certExpirationDelayYears, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageCodeSigning,
		},
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
}
