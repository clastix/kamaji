// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmconfig "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
)

func TestCertificateConfigIsValidFromIssueTime(t *testing.T) {
	initConfiguration, err := kubeadmconfig.DefaultedStaticInitConfiguration()
	require.NoError(t, err)

	config := &Configuration{InitConfiguration: *initConfiguration}
	// Far from the process start time, which kubeadm would use.
	issuedAt := time.Now().Add(10 * 24 * time.Hour)

	testCases := []struct {
		baseName string
		validity time.Duration
	}{
		{baseName: kubeadmconstants.CACertAndKeyBaseName, validity: initConfiguration.CACertificateValidityPeriod.Duration},
		{baseName: kubeadmconstants.FrontProxyCACertAndKeyBaseName, validity: initConfiguration.CACertificateValidityPeriod.Duration},
		{baseName: kubeadmconstants.APIServerCertAndKeyBaseName, validity: initConfiguration.CertificateValidityPeriod.Duration},
		{baseName: kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, validity: initConfiguration.CertificateValidityPeriod.Duration},
		{baseName: kubeadmconstants.FrontProxyClientCertAndKeyBaseName, validity: initConfiguration.CertificateValidityPeriod.Duration},
	}

	for _, tc := range testCases {
		t.Run(tc.baseName, func(t *testing.T) {
			kubeadmCert, err := getKubeadmCert(tc.baseName)
			require.NoError(t, err)

			certConfig, err := certificateConfig(kubeadmCert, config, issuedAt)
			require.NoError(t, err)

			require.True(t, certConfig.NotBefore.Equal(issuedAt.Add(-kubeadmconstants.CertificateBackdate)), "NotBefore: %s", certConfig.NotBefore)
			require.True(t, certConfig.NotAfter.Equal(issuedAt.Add(tc.validity)), "NotAfter: %s", certConfig.NotAfter)
		})
	}
}
