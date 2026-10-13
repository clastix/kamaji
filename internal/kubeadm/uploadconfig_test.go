// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"

	jsonpatchv5 "github.com/evanphx/json-patch/v5"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

func TestGetKubeletConfigmapContentFieldsByTenantVersion(t *testing.T) {
	testCases := []struct {
		version string
		present []string
		absent  []string
	}{
		{
			version: "v1.31.14",
			absent:  []string{"crashLoopBackOff", "mergeDefaultEvictionSettings", "memoryReservationPolicy", "imagePullCredentialsVerificationPolicy"},
		},
		{
			version: "v1.32.13",
			present: []string{"crashLoopBackOff"},
			absent:  []string{"mergeDefaultEvictionSettings", "memoryReservationPolicy", "imagePullCredentialsVerificationPolicy"},
		},
		{
			version: "v1.33.0",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings"},
			absent:  []string{"memoryReservationPolicy", "imagePullCredentialsVerificationPolicy"},
		},
		{
			version: "v1.34.9",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings"},
			absent:  []string{"memoryReservationPolicy", "imagePullCredentialsVerificationPolicy"},
		},
		{
			version: "v1.35.0-rc.1",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings"},
			absent:  []string{"memoryReservationPolicy", "imagePullCredentialsVerificationPolicy"},
		},
		{
			version: "v1.35.6",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings", "imagePullCredentialsVerificationPolicy"},
			absent:  []string{"memoryReservationPolicy"},
		},
		{
			version: "v1.36.0-rc.1",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings", "imagePullCredentialsVerificationPolicy"},
			absent:  []string{"memoryReservationPolicy"},
		},
		{
			version: "v1.36.0",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings", "imagePullCredentialsVerificationPolicy", "memoryReservationPolicy"},
		},
		{
			version: "v1.37.0",
			present: []string{"crashLoopBackOff", "mergeDefaultEvictionSettings", "imagePullCredentialsVerificationPolicy", "memoryReservationPolicy"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.version, func(t *testing.T) {
			content, err := getKubeletConfigmapContent(KubeletConfiguration{}, nil, testCase.version)
			require.NoError(t, err)

			for _, field := range testCase.present {
				require.Regexp(t, `(?m)^`+field+`:`, string(content))
			}

			for _, field := range testCase.absent {
				require.NotRegexp(t, `(?m)^`+field+`:`, string(content))
			}
		})
	}
}

func TestGetKubeletConfigmapContentCrashLoopBackOffByTenantVersion(t *testing.T) {
	for _, version := range []string{"v1.34.9", "v1.35.0-rc.1"} {
		content, err := getKubeletConfigmapContent(KubeletConfiguration{}, nil, version)
		require.NoError(t, err)
		require.Regexp(t, `(?m)^crashLoopBackOff: \{\}$`, string(content))
	}

	content, err := getKubeletConfigmapContent(KubeletConfiguration{}, nil, "v1.35.0")
	require.NoError(t, err)
	require.Regexp(t, `(?m)^crashLoopBackOff:\n  maxContainerRestartPeriod: `, string(content))
}

func TestUploadKubeletConfigRendersForTenantVersion(t *testing.T) {
	client := fake.NewClientset()
	config := &Configuration{Parameters: Parameters{TenantControlPlaneVersion: "v1.35.6"}}

	_, err := UploadKubeletConfig(client, config, nil)
	require.NoError(t, err)

	configMap, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(t.Context(), "kubelet-config", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotRegexp(t, `(?m)^memoryReservationPolicy:`, configMap.Data[kubeadmconstants.KubeletBaseConfigurationConfigMapKey])
}

func TestGetKubeletConfigmapContentDropsUnknownFieldsAfterPatch(t *testing.T) {
	patch, err := jsonpatchv5.DecodePatch([]byte(`[{"op": "add", "path": "/memoryReservationPolicy", "value": "None"}, {"op": "add", "path": "/maxPods", "value": 250}]`))
	require.NoError(t, err)

	content, err := getKubeletConfigmapContent(KubeletConfiguration{}, patch, "v1.35.6")
	require.NoError(t, err)
	require.NotRegexp(t, `(?m)^memoryReservationPolicy:`, string(content))
	require.Regexp(t, `(?m)^maxPods: 250$`, string(content))
}

func TestGetKubeletConfigmapContentInvalidVersion(t *testing.T) {
	_, err := getKubeletConfigmapContent(KubeletConfiguration{}, nil, "not-a-version")
	require.Error(t, err)
}
