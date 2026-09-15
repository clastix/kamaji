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

func TestCheckCertificateAndPrivateKeyPairValidity(t *testing.T) {
	t.Parallel()

	certPEM, keyPEM, err := GenerateSelfSignedCA()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	tests := []struct {
		name      string
		cert      []byte
		key       []byte
		threshold time.Duration
		want      bool
		wantError bool
	}{
		{
			name:      "valid cert and key, well beyond threshold",
			cert:      certPEM,
			key:       keyPEM,
			threshold: 30 * 24 * time.Hour,
			want:      true,
			wantError: false,
		},
		{
			name:      "valid cert and key, within expiration threshold",
			cert:      certPEM,
			key:       keyPEM,
			threshold: 400 * 24 * time.Hour,
			want:      false,
			wantError: false,
		},
		{
			name:      "empty cert bytes",
			cert:      []byte{},
			key:       keyPEM,
			threshold: 30 * 24 * time.Hour,
			want:      false,
			wantError: false,
		},
		{
			name:      "empty key bytes",
			cert:      certPEM,
			key:       []byte{},
			threshold: 30 * 24 * time.Hour,
			want:      false,
			wantError: false,
		},
		{
			name: "mismatched cert and key",
			cert: certPEM,
			key: func() []byte {
				_, k, _ := GenerateSelfSignedCA()

				return k
			}(),
			threshold: 30 * 24 * time.Hour,
			want:      false,
			wantError: false,
		},
		{
			name:      "malformed cert bytes",
			cert:      []byte("not a valid cert"),
			key:       keyPEM,
			threshold: 30 * 24 * time.Hour,
			want:      false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CheckCertificateAndPrivateKeyPairValidity(tt.cert, tt.key, tt.threshold)
			if (err != nil) != tt.wantError {
				t.Errorf("CheckCertificateAndPrivateKeyPairValidity() error = %v, wantError %v", err, tt.wantError)

				return
			}
			if got != tt.want {
				t.Errorf("CheckCertificateAndPrivateKeyPairValidity() = %v, want %v", got, tt.want)
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
