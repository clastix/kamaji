// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestAgentKeepaliveTime(t *testing.T) {
	tests := []struct {
		name      string
		mode      kamajiv1alpha1.KonnectivityAgentMode
		extraArgs kamajiv1alpha1.ExtraArgs
		want      string
	}{
		{
			name: "default for a DaemonSet",
			mode: kamajiv1alpha1.KonnectivityAgentModeDaemonSet,
			want: "60s",
		},
		{
			name: "default for a Deployment",
			mode: kamajiv1alpha1.KonnectivityAgentModeDeployment,
			want: "60s",
		},
		{
			name:      "lowered through extraArgs",
			mode:      kamajiv1alpha1.KonnectivityAgentModeDaemonSet,
			extraArgs: kamajiv1alpha1.ExtraArgs{"--keepalive-time=30s"},
			want:      "30s",
		},
		{
			name:      "previous upstream default restored through extraArgs",
			mode:      kamajiv1alpha1.KonnectivityAgentModeDeployment,
			extraArgs: kamajiv1alpha1.ExtraArgs{"--keepalive-time=1h"},
			want:      "1h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcp := agentTCP(nil)
			tcp.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode = tt.mode
			tcp.Spec.Addons.Konnectivity.KonnectivityAgentSpec.ExtraArgs = tt.extraArgs

			var args []string

			switch tt.mode {
			case kamajiv1alpha1.KonnectivityAgentModeDeployment:
				deploy := &appsv1.Deployment{}
				if err := (&Agent{resource: deploy}).mutate(t.Context(), tcp)(); err != nil {
					t.Fatalf("mutate: %v", err)
				}

				args = deploy.Spec.Template.Spec.Containers[0].Args
			default:
				ds := &appsv1.DaemonSet{}
				if err := (&Agent{resource: ds}).mutate(t.Context(), tcp)(); err != nil {
					t.Fatalf("mutate: %v", err)
				}

				args = ds.Spec.Template.Spec.Containers[0].Args
			}

			var values []string

			for _, arg := range args {
				if value, found := strings.CutPrefix(arg, "--keepalive-time="); found {
					values = append(values, value)
				}
			}

			if len(values) != 1 || values[0] != tt.want {
				t.Errorf("--keepalive-time values = %q, want exactly [%q]", values, tt.want)
			}
		})
	}
}
