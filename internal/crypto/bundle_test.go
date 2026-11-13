// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	"crypto/x509"
	"testing"
)

func newTestBundle(t *testing.T) (newCA, newKey, oldCA, oldKey []byte) {
	t.Helper()

	newCA, newKey, err := GenerateSelfSignedCA()
	if err != nil {
		t.Fatal(err)
	}

	oldCA, oldKey, err = GenerateSelfSignedCA()
	if err != nil {
		t.Fatal(err)
	}

	return newCA, newKey, oldCA, oldKey
}

func TestFirstCertificatePEM(t *testing.T) {
	newCA, _, oldCA, _ := newTestBundle(t)
	bundle := append(append([]byte{}, newCA...), oldCA...)
	withTrailingSpace := append(append([]byte{}, newCA...), "\n\n"...)

	for name, tc := range map[string]struct {
		content  []byte
		expected []byte
		wantErr  bool
	}{
		"single certificate is returned untouched":  {content: newCA, expected: newCA},
		"trailing whitespace is returned untouched": {content: withTrailingSpace, expected: withTrailingSpace},
		"bundle returns its first certificate":      {content: bundle, expected: newCA},
		"empty content":                             {content: nil, wantErr: true},
		"not PEM":                                   {content: []byte("not a certificate"), wantErr: true},
		"not a certificate":                         {content: []byte("-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----\n"), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := FirstCertificatePEM(tc.content)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected error %t, got %v", tc.wantErr, err)
			}

			if !bytes.Equal(actual, tc.expected) {
				t.Fatalf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestValidateCertificateBundle(t *testing.T) {
	newCA, _, oldCA, _ := newTestBundle(t)
	bundle := append(append([]byte{}, newCA...), oldCA...)
	// valid PEM armor with a base64 body the PEM decoder silently skips
	corrupted := append(append([]byte{}, newCA...), "-----BEGIN CERTIFICATE-----\n!!!!\n-----END CERTIFICATE-----\n"...)
	// valid PEM with a body that isn't a certificate
	truncated := append(append([]byte{}, newCA...), "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"...)

	for name, tc := range map[string]struct {
		content []byte
		count   int
		wantErr bool
	}{
		"single certificate":     {content: newCA, count: 1},
		"bundle":                 {content: bundle, count: 2},
		"corrupted second block": {content: corrupted, count: 2, wantErr: true},
		"truncated second block": {content: truncated, count: 2, wantErr: true},
		"no certificate":         {content: []byte("not a certificate"), count: 0, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			if count := CertificateCount(tc.content); count != tc.count {
				t.Fatalf("expected %d certificates, got %d", tc.count, count)
			}

			if err := ValidateCertificateBundle(tc.content); (err != nil) != tc.wantErr {
				t.Fatalf("expected error %t, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestVerifyCertificateWithBundle(t *testing.T) {
	newCA, newKey, oldCA, oldKey := newTestBundle(t)
	bundle := append(append([]byte{}, newCA...), oldCA...)

	oldLeaf, _, err := GenerateCertificatePrivateKeyPair(NewCertificateTemplate("old"), oldCA, oldKey)
	if err != nil {
		t.Fatal(err)
	}

	newLeaf, _, err := GenerateCertificatePrivateKeyPair(NewCertificateTemplate("new"), newCA, newKey)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		leaf     []byte
		ca       []byte
		expected bool
		wantErr  bool
	}{
		"leaf of the old CA is trusted by the bundle":  {leaf: oldLeaf.Bytes(), ca: bundle, expected: true},
		"leaf of the new CA is trusted by the bundle":  {leaf: newLeaf.Bytes(), ca: bundle, expected: true},
		"leaf of the old CA is rejected by the new CA": {leaf: oldLeaf.Bytes(), ca: newCA, wantErr: true},
		"leaf of the new CA is trusted by the new CA":  {leaf: newLeaf.Bytes(), ca: newCA, expected: true},
		"CA content without certificates is an error":  {leaf: newLeaf.Bytes(), ca: []byte("not a certificate"), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := VerifyCertificate(tc.leaf, tc.ca, x509.ExtKeyUsageClientAuth)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected error %t, got %v", tc.wantErr, err)
			}

			if actual != tc.expected {
				t.Fatalf("expected %t, got %t", tc.expected, actual)
			}
		})
	}
}
