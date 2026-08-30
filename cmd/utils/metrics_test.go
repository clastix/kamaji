// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import "testing"

func TestMetricsServerOptions(t *testing.T) {
	t.Parallel()

	insecure := MetricsServerOptions(":8080", false)

	if insecure.BindAddress != ":8080" {
		t.Fatalf("expected bind address to be preserved, got %q", insecure.BindAddress)
	}
	if insecure.SecureServing {
		t.Fatalf("expected SecureServing to be false when secure is false")
	}
	if insecure.FilterProvider != nil {
		t.Fatalf("expected no FilterProvider when secure is false")
	}

	secure := MetricsServerOptions(":8090", true)

	if secure.BindAddress != ":8090" {
		t.Fatalf("expected bind address to be preserved, got %q", secure.BindAddress)
	}
	if !secure.SecureServing {
		t.Fatalf("expected SecureServing to be true when secure is true")
	}
	if secure.FilterProvider == nil {
		t.Fatalf("expected FilterProvider to be set when secure is true")
	}
}
