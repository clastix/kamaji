// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"

	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
)

type Configuration struct {
	InitConfiguration kubeadmapi.InitConfiguration
	Kubeconfig        kubeconfigutil.Kubeconfig
	Parameters        Parameters
}

type Parameters struct {
	TenantControlPlaneName         string
	TenantControlPlaneNamespace    string
	TenantControlPlaneEndpoint     string
	TenantControlPlaneAddress      string
	TenantControlPlaneCertSANs     []string
	TenantControlPlanePort         int32
	TenantControlPlanePodCIDR      string
	TenantControlPlaneServiceCIDR  string
	TenantDNSServiceIPs            []string
	TenantControlPlaneVersion      string
	TenantControlPlaneCGroupDriver string
	ETCDs                          []string
	ETCDCompactionInterval         string
	CertificatesDir                string
	KubeconfigDir                  string
}

type KubeletConfiguration struct {
	TenantControlPlaneDomain        string
	TenantControlPlaneDNSServiceIPs []string
	TenantControlPlaneCgroupDriver  string
}

type CertificatePrivateKeyPair struct {
	Name        string
	Certificate []byte
	PrivateKey  []byte
}

type PublicKeyPrivateKeyPair struct {
	Name       string
	PublicKey  []byte
	PrivateKey []byte
}
