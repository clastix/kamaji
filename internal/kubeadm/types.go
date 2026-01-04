// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	json "github.com/json-iterator/go"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"

	"github.com/clastix/kamaji/internal/utilities"
)

type Configuration struct {
	InitConfiguration kubeadmapi.InitConfiguration
	Kubeconfig        clientcmdapiv1.Config
	Parameters        Parameters
}

func (c *Configuration) Checksum() string {
	initConfiguration, _ := utilities.EncodeToYaml(&c.InitConfiguration)
	kubeconfig, _ := json.Marshal(c.Kubeconfig)
	parameters, _ := json.Marshal(c.Parameters)

	data := map[string][]byte{
		"InitConfiguration": initConfiguration,
		"Kubeconfig":        kubeconfig,
		"Parameters":        parameters,
	}

	return utilities.CalculateMapChecksum(data)
}

type Parameters struct {
	TenantControlPlaneName          string
	TenantControlPlaneNamespace     string
	TenantControlPlaneEndpoint      string
	TenantControlPlaneAddress       string
	TenantControlPlaneCertSANs      []string
	TenantControlPlanePort          int32
	TenantControlPlaneClusterDomain string
	TenantControlPlanePodCIDR       string
	TenantControlPlaneServiceCIDR   string
	TenantDNSServiceIPs             []string
	TenantControlPlaneVersion       string
	TenantControlPlaneCGroupDriver  string
	ETCDs                           []string
	CertificatesDir                 string
	KubeconfigDir                   string
	KubeProxyOptions                *AddonOptions
	CoreDNSOptions                  *AddonOptions
	KubeletFeatureGates             map[string]bool
}

type AddonOptions struct {
	Repository string
	Tag        string
}

type KubeletConfiguration struct {
	TenantControlPlaneDomain        string
	TenantControlPlaneDNSServiceIPs []string
	TenantControlPlaneCgroupDriver  string
	FeatureGates                    map[string]bool
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
