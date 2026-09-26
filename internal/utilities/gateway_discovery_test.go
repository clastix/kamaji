// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"errors"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	k8stesting "k8s.io/client-go/testing"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestMissingGatewayAPIKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		expect    []string
	}{
		{
			name: "all watched kinds are served",
			resources: []*metav1.APIResourceList{{
				GroupVersion: gatewayv1.GroupVersion.String(),
				APIResources: []metav1.APIResource{{Kind: "Gateway"}, {Kind: "HTTPRoute"}, {Kind: "GRPCRoute"}, {Kind: "TLSRoute"}},
			}},
		},
		{
			name: "group is served without TLSRoute",
			resources: []*metav1.APIResourceList{{
				GroupVersion: gatewayv1.GroupVersion.String(),
				APIResources: []metav1.APIResource{{Kind: "Gateway"}, {Kind: "HTTPRoute"}, {Kind: "GRPCRoute"}},
			}},
			expect: []string{"TLSRoute"},
		},
		{
			name:   "group version is not served",
			expect: []string{"Gateway", "HTTPRoute", "GRPCRoute", "TLSRoute"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			missing, err := MissingGatewayAPIKinds(&fakediscovery.FakeDiscovery{Fake: &k8stesting.Fake{Resources: tt.resources}})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(missing, tt.expect) {
				t.Fatalf("want missing kinds %v, got %v", tt.expect, missing)
			}
		})
	}
}

func TestMissingGatewayAPIKinds_DiscoveryError(t *testing.T) {
	t.Parallel()

	discoveryErr := errors.New("connection refused")

	fake := &k8stesting.Fake{}
	fake.AddReactor("get", "resource", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, discoveryErr
	})

	if _, err := MissingGatewayAPIKinds(&fakediscovery.FakeDiscovery{Fake: fake}); !errors.Is(err, discoveryErr) {
		t.Fatalf("a discovery failure must not be reported as missing kinds, got error %v", err)
	}
}
