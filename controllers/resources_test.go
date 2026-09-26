// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/clastix/kamaji/internal/datastore"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

// countingRESTMapper counts RESTMapper lookups, the route by which a controller-runtime client reaches discovery.
type countingRESTMapper struct {
	meta.RESTMapper

	calls int
}

func (m *countingRESTMapper) KindFor(r schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	m.calls++

	return m.RESTMapper.KindFor(r)
}

func (m *countingRESTMapper) KindsFor(r schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	m.calls++

	return m.RESTMapper.KindsFor(r)
}

func (m *countingRESTMapper) ResourceFor(r schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	m.calls++

	return m.RESTMapper.ResourceFor(r)
}

func (m *countingRESTMapper) ResourcesFor(r schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	m.calls++

	return m.RESTMapper.ResourcesFor(r)
}

func (m *countingRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	m.calls++

	return m.RESTMapper.RESTMapping(gk, versions...)
}

func (m *countingRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	m.calls++

	return m.RESTMapper.RESTMappings(gk, versions...)
}

type stubConnection struct {
	datastore.Connection
}

func (stubConnection) GetConnectionString() string { return "" }

func TestGetResources_ConsultsNoRESTMapper(t *testing.T) {
	t.Parallel()

	for _, available := range []bool{true, false} {
		mapper := &countingRESTMapper{RESTMapper: meta.NewDefaultRESTMapper(nil)}
		config := GroupResourceBuilderConfiguration{
			client:              fake.NewClientBuilder().WithRESTMapper(mapper).Build(),
			Connection:          stubConnection{},
			GatewayAPIAvailable: available,
		}

		GetResources(context.Background(), config)

		if mapper.calls != 0 {
			t.Fatalf("GatewayAPIAvailable=%t: GetResources runs inside the reconcile timeout and must not consult the RESTMapper, got %d lookups", available, mapper.calls)
		}
	}
}

func TestGetResources_GatewayResourcesFollowAvailability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		available bool
	}{
		{name: "gateway api available", available: true},
		{name: "gateway api not available", available: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := GroupResourceBuilderConfiguration{
				client:              fake.NewClientBuilder().Build(),
				Connection:          stubConnection{},
				GatewayAPIAvailable: tt.available,
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

			if kubernetesGateway != tt.available || konnectivityGateway != tt.available {
				t.Fatalf("want gateway resources present=%t, got KubernetesGatewayResource=%t KubernetesKonnectivityGatewayResource=%t", tt.available, kubernetesGateway, konnectivityGateway)
			}
		})
	}
}
