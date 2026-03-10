// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"

	kubelettypes "k8s.io/kubelet/config/v1beta1"

	"github.com/clastix/kamaji/internal/utilities"
)

func TestGetKubeletConfigmapContent_VersionGatedFields(t *testing.T) {
	t.Parallel()

	kubeletCfg := KubeletConfiguration{
		TenantControlPlaneDomain:        "cluster.local",
		TenantControlPlaneDNSServiceIPs: []string{"10.96.0.10"},
		TenantControlPlaneCgroupDriver:  "systemd",
	}

	tests := []struct {
		name                                   string
		version                                string
		expectCrashLoopBackOffCleared           bool
		expectImagePullCredentialsPolicyCleared bool
	}{
		{
			name:                                   "v1.30 should clear version-gated fields",
			version:                                "1.30.0",
			expectCrashLoopBackOffCleared:           true,
			expectImagePullCredentialsPolicyCleared: true,
		},
		{
			name:                                   "v1.34.5 should clear version-gated fields",
			version:                                "1.34.5",
			expectCrashLoopBackOffCleared:           true,
			expectImagePullCredentialsPolicyCleared: true,
		},
		{
			name:                                   "v-prefixed v1.34.1 should clear version-gated fields",
			version:                                "v1.34.1",
			expectCrashLoopBackOffCleared:           true,
			expectImagePullCredentialsPolicyCleared: true,
		},
		{
			name:                                   "v1.35.0 should preserve version-gated fields",
			version:                                "1.35.0",
			expectCrashLoopBackOffCleared:           false,
			expectImagePullCredentialsPolicyCleared: false,
		},
		{
			name:                                   "v1.36.0 should preserve version-gated fields",
			version:                                "1.36.0",
			expectCrashLoopBackOffCleared:           false,
			expectImagePullCredentialsPolicyCleared: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := getKubeletConfigmapContent(kubeletCfg, nil, tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if content == nil {
				t.Fatal("expected non-nil content")
			}

			var kc kubelettypes.KubeletConfiguration
			if err := utilities.DecodeFromYAML(string(content), &kc); err != nil {
				t.Fatalf("failed to decode kubelet config: %v", err)
			}

			if tt.expectCrashLoopBackOffCleared {
				if kc.CrashLoopBackOff.MaxContainerRestartPeriod != nil {
					t.Errorf("expected CrashLoopBackOff.MaxContainerRestartPeriod to be nil for version %s, got %v",
						tt.version, kc.CrashLoopBackOff.MaxContainerRestartPeriod)
				}
			} else {
				if kc.CrashLoopBackOff.MaxContainerRestartPeriod == nil {
					t.Errorf("expected CrashLoopBackOff.MaxContainerRestartPeriod to be set for version %s",
						tt.version)
				}
			}

			if tt.expectImagePullCredentialsPolicyCleared {
				if kc.ImagePullCredentialsVerificationPolicy != "" {
					t.Errorf("expected ImagePullCredentialsVerificationPolicy to be empty for version %s, got %q",
						tt.version, kc.ImagePullCredentialsVerificationPolicy)
				}
			} else {
				if kc.ImagePullCredentialsVerificationPolicy == "" {
					t.Errorf("expected ImagePullCredentialsVerificationPolicy to be set for version %s",
						tt.version)
				}
			}
		})
	}
}

func TestGetKubeletConfigmapContent_InvalidVersion(t *testing.T) {
	t.Parallel()

	kubeletCfg := KubeletConfiguration{
		TenantControlPlaneDomain:        "cluster.local",
		TenantControlPlaneDNSServiceIPs: []string{"10.96.0.10"},
		TenantControlPlaneCgroupDriver:  "systemd",
	}

	invalidVersions := []string{"", "not-a-version", "abc.def.ghi"}

	for _, version := range invalidVersions {
		t.Run("version="+version, func(t *testing.T) {
			t.Parallel()

			_, err := getKubeletConfigmapContent(kubeletCfg, nil, version)
			if err == nil {
				t.Errorf("expected error for invalid version %q, got nil", version)
			}
		})
	}
}
