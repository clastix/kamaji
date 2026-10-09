// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsRotationRequested(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name:        "annotation key absent",
			annotations: map[string]string{"other.key": "value"},
			want:        false,
		},
		{
			name:        "annotation present with empty value",
			annotations: map[string]string{RotateCertificateRequestAnnotation: ""},
			want:        true,
		},
		{
			name:        "annotation present with timestamp value",
			annotations: map[string]string{RotateCertificateRequestAnnotation: "2025-01-15T10:00:00Z"},
			want:        false,
		},
		{
			name:        "annotation present with arbitrary non-empty value",
			annotations: map[string]string{RotateCertificateRequestAnnotation: "some-value"},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}
			got := IsRotationRequested(secret)
			if got != tt.want {
				t.Errorf("IsRotationRequested() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetLastRotationTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		initialAnnotations       map[string]string
		expectAnnotationPresent  bool
		expectTimestampParseable bool
		expectOtherAnnotations   map[string]string
	}{
		{
			name:                     "nil annotations",
			initialAnnotations:       nil,
			expectAnnotationPresent:  true,
			expectTimestampParseable: true,
			expectOtherAnnotations:   map[string]string{},
		},
		{
			name:                     "empty annotations",
			initialAnnotations:       map[string]string{},
			expectAnnotationPresent:  true,
			expectTimestampParseable: true,
			expectOtherAnnotations:   map[string]string{},
		},
		{
			name:                     "existing annotations preserved",
			initialAnnotations:       map[string]string{"other.key": "other.value"},
			expectAnnotationPresent:  true,
			expectTimestampParseable: true,
			expectOtherAnnotations:   map[string]string{"other.key": "other.value"},
		},
		{
			name:                     "overwrites existing rotation timestamp",
			initialAnnotations:       map[string]string{RotateCertificateRequestAnnotation: "2025-01-14T10:00:00Z"},
			expectAnnotationPresent:  true,
			expectTimestampParseable: true,
			expectOtherAnnotations:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   "default",
					Annotations: tt.initialAnnotations,
				},
			}

			SetLastRotationTimestamp(secret)

			annotations := secret.GetAnnotations()
			if !tt.expectAnnotationPresent {
				t.Errorf("SetLastRotationTimestamp() annotation not present")

				return
			}

			value, ok := annotations[RotateCertificateRequestAnnotation]
			if !ok {
				t.Errorf("SetLastRotationTimestamp() annotation key not found")

				return
			}

			if value == "" {
				t.Errorf("SetLastRotationTimestamp() annotation value is empty, expected timestamp")

				return
			}

			if tt.expectTimestampParseable {
				_, err := time.Parse(time.RFC3339, value)
				if err != nil {
					t.Errorf("SetLastRotationTimestamp() annotation value %q not parseable as RFC3339: %v", value, err)

					return
				}
			}

			for k, v := range tt.expectOtherAnnotations {
				if annotations[k] != v {
					t.Errorf("SetLastRotationTimestamp() lost or changed annotation %s: expected %q, got %q", k, v, annotations[k])
				}
			}
		})
	}
}
