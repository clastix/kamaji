// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/clastix/kamaji/internal/datastore"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

type stubConnection struct {
	datastore.Connection
}

func (stubConnection) GetConnectionString() string { return "" }

func TestGetResources_GatewayResourcesFollowFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enabled bool
	}{
		{name: "gateway api enabled", enabled: true},
		{name: "gateway api disabled", enabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := GroupResourceBuilderConfiguration{
				client:            fake.NewClientBuilder().Build(),
				Connection:        stubConnection{},
				GatewayAPIEnabled: tt.enabled,
			}

			var kubernetesGateway, konnectivityGateway bool

			for _, resource := range GetResources(context.Background(), config) {
				switch resource.(type) {
				case *resources.KubernetesGatewayResource:
					kubernetesGateway = true
				case *konnectivity.KubernetesKonnectivityGatewayResource:
					konnectivityGateway = true
				}
			}

			if kubernetesGateway != tt.enabled || konnectivityGateway != tt.enabled {
				t.Fatalf("want gateway resources present=%t, got KubernetesGatewayResource=%t KubernetesKonnectivityGatewayResource=%t", tt.enabled, kubernetesGateway, konnectivityGateway)
			}
		})
	}
}
