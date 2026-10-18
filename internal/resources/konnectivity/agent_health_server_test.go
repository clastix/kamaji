// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestAgentHealthServer(t *testing.T) {
	tests := []struct {
		name        string
		hostNetwork bool
		extraArgs   kamajiv1alpha1.ExtraArgs
		wantHost    string
		wantPort    intstr.IntOrString
		wantProbe   string
	}{
		{
			name:     "pod network keeps every interface of the Pod",
			wantPort: intstr.FromInt32(8134),
		},
		{
			name:        "host network binds to loopback",
			hostNetwork: true,
			wantHost:    "127.0.0.1",
			wantPort:    intstr.FromInt32(8134),
			wantProbe:   "127.0.0.1",
		},
		{
			name:        "every interface requested through extraArgs",
			hostNetwork: true,
			extraArgs:   kamajiv1alpha1.ExtraArgs{"--health-server-host=0.0.0.0"},
			wantHost:    "0.0.0.0",
			wantPort:    intstr.FromInt32(8134),
		},
		{
			name:        "address and port overridden through extraArgs",
			hostNetwork: true,
			extraArgs:   kamajiv1alpha1.ExtraArgs{"--health-server-host=10.0.0.10", "--health-server-port=9134"},
			wantHost:    "10.0.0.10",
			wantPort:    intstr.FromInt32(9134),
			wantProbe:   "10.0.0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcp := agentTCP(nil)
			tcp.Spec.Addons.Konnectivity.KonnectivityAgentSpec.HostNetwork = tt.hostNetwork
			tcp.Spec.Addons.Konnectivity.KonnectivityAgentSpec.ExtraArgs = tt.extraArgs

			ds := &appsv1.DaemonSet{}
			if err := (&Agent{resource: ds}).mutate(t.Context(), tcp)(); err != nil {
				t.Fatalf("mutate: %v", err)
			}

			container := ds.Spec.Template.Spec.Containers[0]

			host, set := "", false
			for _, arg := range container.Args {
				if value, found := strings.CutPrefix(arg, "--health-server-host="); found {
					host, set = value, true
				}
			}

			if host != tt.wantHost || set != (tt.wantHost != "") {
				t.Errorf("--health-server-host = %q (set: %t), want %q", host, set, tt.wantHost)
			}

			probe := container.LivenessProbe.HTTPGet
			if probe.Host != tt.wantProbe {
				t.Errorf("liveness probe host = %q, want %q", probe.Host, tt.wantProbe)
			}

			if probe.Port != tt.wantPort {
				t.Errorf("liveness probe port = %s, want %s", probe.Port.String(), tt.wantPort.String())
			}
		})
	}
}
