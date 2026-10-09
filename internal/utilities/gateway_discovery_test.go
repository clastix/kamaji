// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func gatewayResources(kinds ...string) []*metav1.APIResourceList {
	list := &metav1.APIResourceList{GroupVersion: gatewayv1.GroupVersion.String()}
	for _, kind := range kinds {
		list.APIResources = append(list.APIResources, metav1.APIResource{Kind: kind})
	}

	return []*metav1.APIResourceList{list}
}

func TestAreGatewayResourcesAvailable(t *testing.T) {
	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		expect    bool
	}{
		{
			name:   "group not served",
			expect: false,
		},
		{
			name:      "Gateway and TLSRoute served",
			resources: gatewayResources("Gateway", "HTTPRoute", "TLSRoute"),
			expect:    true,
		},
		// Serving the group does not mean serving every kind Kamaji watches; an informer for an
		// unserved kind never syncs and the manager fails to start.
		{
			name:      "group served without TLSRoute",
			resources: gatewayResources("Gateway", "HTTPRoute"),
			expect:    false,
		},
		{
			name:      "group served without Gateway",
			resources: gatewayResources("HTTPRoute", "TLSRoute"),
			expect:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: tt.resources}}

			if got := AreGatewayResourcesAvailable(context.Background(), nil, d); got != tt.expect {
				t.Errorf("AreGatewayResourcesAvailable() = %v, want %v", got, tt.expect)
			}
		})
	}
}
