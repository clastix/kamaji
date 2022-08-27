// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	json "github.com/json-iterator/go"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"

	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
	"github.com/clastix/kamaji/internal/utilities"
)

type Configuration struct {
	InitConfiguration kubeadmapi.InitConfiguration
	Kubeconfig        kubeconfigutil.Kubeconfig
	Parameters        Parameters
}

func (c *Configuration) Checksum() string {
	initConfiguration, _ := utilities.EncondeToYaml(&c.InitConfiguration)
	kubeconfig, _ := json.Marshal(c.Kubeconfig)
	parameters, _ := json.Marshal(c.Parameters)

	data := map[string]string{
		"InitConfiguration": string(initConfiguration),
		"Kubeconfig":        string(kubeconfig),
		"Parameters":        string(parameters),
	}

	return utilities.CalculateConfigMapChecksum(data)
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
	CertificatesDir                string
	KubeconfigDir                  string
	KubeProxyImage                 string
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
