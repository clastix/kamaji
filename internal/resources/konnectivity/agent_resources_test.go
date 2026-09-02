// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// agentTCP returns the minimum TenantControlPlane the agent mutate function
// needs: an advertised address it can resolve, and a Konnectivity addon.
func agentTCP(res *corev1.ResourceRequirements) *kamajiv1alpha1.TenantControlPlane {
	tcp := &kamajiv1alpha1.TenantControlPlane{}
	tcp.SetName("test-tcp")
	tcp.SetNamespace("default")
	tcp.Spec.NetworkProfile.AdvertiseAddress = "192.168.0.1"
	tcp.Spec.NetworkProfile.Port = 6443
	tcp.Status.ControlPlaneEndpoint = "192.168.0.1:6443"
	tcp.Spec.Addons.Konnectivity = &kamajiv1alpha1.KonnectivitySpec{
		KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{Port: 8132},
		KonnectivityAgentSpec: kamajiv1alpha1.KonnectivityAgentSpec{
			Image:     "registry.k8s.io/kas-network-proxy/proxy-agent",
			Version:   "v0.28.0",
			Mode:      kamajiv1alpha1.KonnectivityAgentModeDaemonSet,
			Resources: res,
		},
	}

	return tcp
}

func agentContainerResources(t *testing.T, tcp *kamajiv1alpha1.TenantControlPlane) corev1.ResourceRequirements {
	t.Helper()

	ds := &appsv1.DaemonSet{}
	r := &Agent{resource: ds}

	if err := r.mutate(context.Background(), tcp)(); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	if len(ds.Spec.Template.Spec.Containers) == 0 {
		t.Fatal("mutate produced no containers")
	}

	return ds.Spec.Template.Spec.Containers[0].Resources
}

// TestAgentResources_Unset asserts the previous behaviour is preserved: with no
// Resources declared the container carries neither requests nor limits, leaving
// the Pod BestEffort exactly as before this field existed.
func TestAgentResources_Unset(t *testing.T) {
	got := agentContainerResources(t, agentTCP(nil))

	if got.Requests != nil {
		t.Errorf("requests = %v, want nil when Resources is unset", got.Requests)
	}

	if got.Limits != nil {
		t.Errorf("limits = %v, want nil when Resources is unset", got.Limits)
	}
}

// TestAgentResources_Applied asserts the declared Resources reach the agent
// container. Without this the field is accepted by the API and then silently
// dropped, which is indistinguishable from it working.
func TestAgentResources_Applied(t *testing.T) {
	want := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	got := agentContainerResources(t, agentTCP(want))

	if cpu := got.Requests[corev1.ResourceCPU]; cpu.String() != "100m" {
		t.Errorf("cpu request = %q, want 100m", cpu.String())
	}

	if mem := got.Requests[corev1.ResourceMemory]; mem.String() != "64Mi" {
		t.Errorf("memory request = %q, want 64Mi", mem.String())
	}

	if mem := got.Limits[corev1.ResourceMemory]; mem.String() != "256Mi" {
		t.Errorf("memory limit = %q, want 256Mi", mem.String())
	}
}

// TestAgentResources_RequestsOnly covers the case this field mainly exists for:
// requests without limits, which promotes the agent from BestEffort to Burstable
// without introducing a CPU-throttling or OOMKill path on a component the
// exec/logs tunnel depends on.
func TestAgentResources_RequestsOnly(t *testing.T) {
	got := agentContainerResources(t, agentTCP(&corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}))

	if cpu := got.Requests[corev1.ResourceCPU]; cpu.String() != "100m" {
		t.Errorf("cpu request = %q, want 100m", cpu.String())
	}

	if got.Limits != nil {
		t.Errorf("limits = %v, want nil so the container stays Burstable without a throttling ceiling", got.Limits)
	}
}
