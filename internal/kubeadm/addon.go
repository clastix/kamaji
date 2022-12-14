// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"bytes"

	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
)

const (
	KubeSystemNamespace = "kube-system"

	KubeProxyName               = constants.KubeProxy
	KubeProxyServiceAccountName = proxy.KubeProxyServiceAccountName
	KubeProxyConfigMapRoleName  = proxy.KubeProxyConfigMapRoleName
	KubeProxyConfigMap          = constants.KubeProxyConfigMap

	CoreDNSName                   = constants.CoreDNSDeploymentName
	CoreDNSServiceName            = "kube-dns"
	CoreDNSClusterRoleName        = "system:coredns"
	CoreDNSClusterRoleBindingName = "system:coredns"
)

func AddCoreDNS(client kubernetes.Interface, config *Configuration) ([]byte, error) {
	// We're passing the values from the parameters here because they wouldn't be hashed by the YAML encoder:
	// the struct kubeadm.ClusterConfiguration hasn't struct tags, and it wouldn't be hashed properly.
	if opts := config.Parameters.CoreDNSOptions; opts != nil {
		config.InitConfiguration.DNS.ImageRepository = opts.Repository
		config.InitConfiguration.DNS.ImageTag = opts.Tag
	}

	b := bytes.NewBuffer([]byte{})
	if err := dns.EnsureDNSAddon(&config.InitConfiguration.ClusterConfiguration, client, b, true); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func AddKubeProxy(client kubernetes.Interface, config *Configuration) ([]byte, error) {
	// This is a workaround since the function EnsureProxyAddon is picking repository and tag from the InitConfiguration
	// struct, although is counterintuitive
	config.InitConfiguration.ClusterConfiguration.CIImageRepository = config.Parameters.KubeProxyOptions.Repository
	config.InitConfiguration.KubernetesVersion = config.Parameters.KubeProxyOptions.Tag

	b := bytes.NewBuffer([]byte{})
	if err := proxy.EnsureProxyAddon(&config.InitConfiguration.ClusterConfiguration, &config.InitConfiguration.LocalAPIEndpoint, client, b, true); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
