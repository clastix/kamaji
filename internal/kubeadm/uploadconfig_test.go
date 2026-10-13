// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"

	jsonpatchv5 "github.com/evanphx/json-patch/v5"
	"github.com/stretchr/testify/require"
)

func TestGetKubeletConfigmapContentJSONPatches(t *testing.T) {
	testCases := []struct {
		name    string
		patch   string
		present []string
		absent  []string
	}{
		{
			name:    "replace",
			patch:   `[{"op": "replace", "path": "/cgroupDriver", "value": "systemd"}]`,
			present: []string{`cgroupDriver: systemd`},
		},
		{
			name:   "remove a struct field",
			patch:  `[{"op": "remove", "path": "/crashLoopBackOff"}]`,
			absent: []string{`crashLoopBackOff:`},
		},
		{
			name:    "remove a field that is not emitted",
			patch:   `[{"op": "remove", "path": "/defaultPodSysctls"}, {"op": "add", "path": "/maxPods", "value": 250}]`,
			present: []string{`maxPods: 250`},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			patch, err := jsonpatchv5.DecodePatch([]byte(testCase.patch))
			require.NoError(t, err)

			content, err := getKubeletConfigmapContent(KubeletConfiguration{TenantControlPlaneCgroupDriver: "cgroupfs"}, patch)
			require.NoError(t, err)

			for _, line := range testCase.present {
				require.Regexp(t, `(?m)^`+line+`$`, string(content))
			}

			for _, line := range testCase.absent {
				require.NotRegexp(t, `(?m)^`+line, string(content))
			}
		})
	}
}

func TestGetKubeletConfigmapContentRejectsPatchWithWrongType(t *testing.T) {
	patch, err := jsonpatchv5.DecodePatch([]byte(`[{"op": "replace", "path": "/maxPods", "value": "many"}]`))
	require.NoError(t, err)

	_, err = getKubeletConfigmapContent(KubeletConfiguration{}, patch)
	require.Error(t, err)
}

func TestGetKubeletConfigmapContentNoOpPatchKeepsOutput(t *testing.T) {
	kubeletConfiguration := KubeletConfiguration{TenantControlPlaneCgroupDriver: "cgroupfs"}

	patch, err := jsonpatchv5.DecodePatch([]byte(`[{"op": "replace", "path": "/cgroupDriver", "value": "cgroupfs"}]`))
	require.NoError(t, err)

	unpatched, err := getKubeletConfigmapContent(kubeletConfiguration, nil)
	require.NoError(t, err)

	patched, err := getKubeletConfigmapContent(kubeletConfiguration, patch)
	require.NoError(t, err)

	require.Equal(t, string(unpatched), string(patched))
}
