// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"fmt"
	"strings"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/config"

	"github.com/clastix/kamaji/internal/utilities"
)

const (
	defaultCAFile   = "/etc/kubernetes/pki/etcd/ca.crt"
	defaultCertFile = "/etc/kubernetes/pki/apiserver-etcd-client.crt"
	defaultKeyFile  = "/etc/kubernetes/pki/apiserver-etcd-client.key"
)

func CreateKubeadmInitConfiguration(params Parameters) (*Configuration, error) {
	defaultConf, err := config.DefaultedStaticInitConfiguration()
	if err != nil {
		return nil, err
	}

	conf := defaultConf
	// Due to unmarshaling error when GetKubeadmInitConfigurationFromMap function is issued,
	// we have to store the ComponentConfigs to a null value.
	conf.ClusterConfiguration.ComponentConfigs = nil

	conf.LocalAPIEndpoint = kubeadmapi.APIEndpoint{
		AdvertiseAddress: params.TenantControlPlaneAddress,
		BindPort:         params.TenantControlPlanePort,
	}

	caFile, certFile, keyFile := "", "", ""
	if strings.HasPrefix(params.ETCDs[0], "https") {
		caFile, certFile, keyFile = defaultCAFile, defaultCertFile, defaultKeyFile
	}

	conf.Etcd = kubeadmapi.Etcd{
		External: &kubeadmapi.ExternalEtcd{
			Endpoints: params.ETCDs,
			CAFile:    caFile,
			CertFile:  certFile,
			KeyFile:   keyFile,
		},
	}
	conf.Networking = kubeadmapi.Networking{
		DNSDomain:     params.TenantControlPlaneClusterDomain,
		PodSubnet:     params.TenantControlPlanePodCIDR,
		ServiceSubnet: params.TenantControlPlaneServiceCIDR,
	}
	conf.KubernetesVersion = params.TenantControlPlaneVersion
	conf.ControlPlaneEndpoint = params.TenantControlPlaneEndpoint
	conf.APIServer.CertSANs = append([]string{
		"127.0.0.1",
		"localhost",
		params.TenantControlPlaneName,
		fmt.Sprintf("%s.%s.svc", params.TenantControlPlaneName, params.TenantControlPlaneNamespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", params.TenantControlPlaneName, params.TenantControlPlaneNamespace),
		params.TenantControlPlaneAddress,
	}, params.TenantControlPlaneCertSANs...)
	conf.APIServer.ControlPlaneComponent.ExtraArgs = []kubeadmapi.Arg{
		{Name: "etcd-compaction-interval", Value: "0s"},
		{Name: "etcd-prefix", Value: fmt.Sprintf("/%s", params.TenantControlPlaneName)},
	}
	conf.ClusterName = params.TenantControlPlaneName

	return &Configuration{InitConfiguration: *conf}, nil
}

func GetKubeadmInitConfigurationMap(config Configuration) (map[string]string, error) {
	initConfigurationString, err := utilities.EncodeToJSON(&config.InitConfiguration)
	if err != nil {
		return nil, err
	}

	clusterConfigurationString, err := utilities.EncodeToJSON(&config.InitConfiguration.ClusterConfiguration)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		kubeadmconstants.InitConfigurationKind:    string(initConfigurationString),
		kubeadmconstants.ClusterConfigurationKind: string(clusterConfigurationString),
	}, nil
}

func GetKubeadmInitConfigurationFromMap(conf map[string]string) (*Configuration, error) {
	initConfigurationString, ok := conf[kubeadmconstants.InitConfigurationKind]
	if !ok {
		return nil, fmt.Errorf("%s is not in the map", kubeadmconstants.InitConfigurationKind)
	}

	clusterConfigurationString, ok := conf[kubeadmconstants.ClusterConfigurationKind]
	if !ok {
		return nil, fmt.Errorf("%s is not in the map", kubeadmconstants.ClusterConfigurationKind)
	}

	initConfiguration := kubeadmapi.InitConfiguration{}
	if err := utilities.DecodeFromJSON(initConfigurationString, &initConfiguration); err != nil {
		return nil, err
	}

	if err := utilities.DecodeFromJSON(clusterConfigurationString, &initConfiguration.ClusterConfiguration); err != nil {
		return nil, err
	}
	// Due to some weird issues with unmarshaling of the ComponentConfigs struct,
	// we have to extract the default value and assign it directly.
	defaults, err := config.DefaultedStaticInitConfiguration()
	if err != nil {
		return nil, err
	}
	initConfiguration.ClusterConfiguration.ComponentConfigs = defaults.ComponentConfigs

	return &Configuration{InitConfiguration: initConfiguration}, nil
}
