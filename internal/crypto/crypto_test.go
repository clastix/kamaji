// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestParsePrivateKeyBytes(t *testing.T) {
	tests := []struct {
		name      string
		keyType   string
		keyGen    func() []byte
		wantError bool
	}{
		{
			name:    "PKCS1 RSA key",
			keyType: "PKCS1",
			keyGen: func() []byte {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					t.Fatalf("failed to generate RSA key: %v", err)
				}
				keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

				return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
			},
			wantError: false,
		},
		{
			name:    "PKCS8 RSA key",
			keyType: "PKCS8",
			keyGen: func() []byte {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					t.Fatalf("failed to generate RSA key: %v", err)
				}
				pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
				if err != nil {
					t.Fatalf("failed to marshal PKCS8: %v", err)
				}

				return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
			},
			wantError: false,
		},
		{
			name:    "EC key",
			keyType: "EC",
			keyGen: func() []byte {
				privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					t.Fatalf("failed to generate EC key: %v", err)
				}
				ecBytes, err := x509.MarshalECPrivateKey(privateKey)
				if err != nil {
					t.Fatalf("failed to marshal EC: %v", err)
				}

				return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecBytes})
			},
			wantError: false,
		},
		{
			name:    "invalid key",
			keyType: "invalid",
			keyGen: func() []byte {
				return pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN", Bytes: []byte("not a valid key")})
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyBytes := tt.keyGen()
			_, err := ParsePrivateKeyBytes(keyBytes)
			if (err != nil) != tt.wantError {
				t.Errorf("ParsePrivateKeyBytes() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func GenerateSelfSignedCA() ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return certPEM, keyPEM, nil
}
