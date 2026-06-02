// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities_test

import (
	"crypto/md5"
	"encoding/hex"
	"testing"

	"github.com/clastix/kamaji/internal/utilities"
)

func TestCalculateStringSliceChecksum(t *testing.T) {
	tests := []struct {
		name     string
		data     []string
		expected string
	}{
		{
			name:     "empty slice",
			data:     []string{},
			expected: md5Hex([]byte{}),
		},
		{
			name:     "single element",
			data:     []string{"hello"},
			expected: md5Hex([]byte("hello\x00")),
		},
		{
			name:     "multiple elements",
			data:     []string{"a", "b", "c"},
			expected: md5Hex([]byte("a\x00b\x00c\x00")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utilities.CalculateStringSliceChecksum(tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("CalculateStringSliceChecksum(%v) = %q, want %q", tt.data, got, tt.expected)
			}
		})
	}
}

func TestCalculateStringSliceChecksumOrder(t *testing.T) {
	// Test that different orders produce different checksums
	a, err := utilities.CalculateStringSliceChecksum([]string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := utilities.CalculateStringSliceChecksum([]string{"b", "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if a == b {
		t.Errorf("expected different checksums for [a,b] and [b,a], but got %q and %q", a, b)
	}
}

func TestCalculateStringSliceChecksumCollision(t *testing.T) {
	// Test that ["ab", "c"] and ["a", "bc"] produce different checksums
	a, err := utilities.CalculateStringSliceChecksum([]string{"ab", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := utilities.CalculateStringSliceChecksum([]string{"a", "bc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if a == b {
		t.Errorf("expected different checksums for [ab,c] and [a,bc], but got %q and %q", a, b)
	}
}

func md5Hex(data []byte) string {
	hash := md5.Sum(data)

	return hex.EncodeToString(hash[:])
}
