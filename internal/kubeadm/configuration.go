// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

const (
	defaultCAFile   = "/etc/kubernetes/pki/etcd/ca.crt"
	defaultCertFile = "/etc/kubernetes/pki/apiserver-etcd-client.crt"
	defaultKeyFile  = "/etc/kubernetes/pki/apiserver-etcd-client.key"
)

func CreateKubeadmInitConfiguration(params Parameters) Configuration {
	config := kubeadmapi.InitConfiguration{
		ClusterConfiguration: getKubeadmClusterConfiguration(params),
		BootstrapTokens: []bootstraptokenv1.BootstrapToken{
			{
				Groups: []string{"system:bootstrappers:kubeadm:default-node-token"},
				TTL:    &metav1.Duration{Duration: 48 * time.Hour},
				Usages: []string{
					"signing",
					"authentication",
				},
			},
		},
		LocalAPIEndpoint: kubeadmapi.APIEndpoint{
			AdvertiseAddress: params.TenantControlPlaneAddress,
			BindPort:         params.TenantControlPlanePort,
		},
		NodeRegistration: kubeadmapi.NodeRegistrationOptions{
			CRISocket: "unix:///run/containerd/containerd.sock",
			Name:      params.TenantControlPlaneName,
		},
	}

	return Configuration{InitConfiguration: config}
}

func isHTTPS(url string) bool {
	return strings.HasPrefix(url, "https")
}

func getKubeadmClusterConfiguration(params Parameters) kubeadmapi.ClusterConfiguration {
	caFile, certFile, keyFile := "", "", ""
	if isHTTPS(params.ETCDs[0]) {
		caFile, certFile, keyFile = defaultCAFile, defaultCertFile, defaultKeyFile
	}

	return kubeadmapi.ClusterConfiguration{
		KubernetesVersion: params.TenantControlPlaneVersion,
		ClusterName:       params.TenantControlPlaneName,
		CertificatesDir:   "/etc/kubernetes/pki",
		ImageRepository:   "k8s.gcr.io",
		Networking: kubeadmapi.Networking{
			DNSDomain:     "cluster.local",
			PodSubnet:     params.TenantControlPlanePodCIDR,
			ServiceSubnet: params.TenantControlPlaneServiceCIDR,
		},
		DNS: kubeadmapi.DNS{
			Type: "CoreDNS",
		},
		ControlPlaneEndpoint: params.TenantControlPlaneEndpoint,
		Etcd: kubeadmapi.Etcd{
			External: &kubeadmapi.ExternalEtcd{
				Endpoints: params.ETCDs,
				CAFile:    caFile,
				CertFile:  certFile,
				KeyFile:   keyFile,
			},
		},
		APIServer: kubeadmapi.APIServer{
			CertSANs: append([]string{
				"127.0.0.1",
				"localhost",
				params.TenantControlPlaneName,
				fmt.Sprintf("%s.%s.svc", params.TenantControlPlaneName, params.TenantControlPlaneNamespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", params.TenantControlPlaneName, params.TenantControlPlaneNamespace),
				params.TenantControlPlaneAddress,
			}, params.TenantControlPlaneCertSANs...),
			ControlPlaneComponent: kubeadmapi.ControlPlaneComponent{
				ExtraArgs: map[string]string{
					"etcd-compaction-interval": params.ETCDCompactionInterval,
					"etcd-prefix":              fmt.Sprintf("/%s", params.TenantControlPlaneName),
				},
			},
		},
	}
}

func GetKubeadmInitConfigurationMap(config Configuration) (map[string]string, error) {
	initConfigurationString, err := getJSONStringFromStruct(config.InitConfiguration)
	if err != nil {
		return map[string]string{}, err
	}

	clusterConfigurationString, err := getJSONStringFromStruct(config.InitConfiguration.ClusterConfiguration)
	if err != nil {
		return map[string]string{}, err
	}

	return map[string]string{
		kubeadmconstants.InitConfigurationKind:    initConfigurationString,
		kubeadmconstants.ClusterConfigurationKind: clusterConfigurationString,
	}, nil
}

func GetKubeadmInitConfigurationFromMap(config map[string]string) (*Configuration, error) {
	initConfigurationString, ok := config[kubeadmconstants.InitConfigurationKind]
	if !ok {
		return nil, fmt.Errorf("%s is not in the map", kubeadmconstants.InitConfigurationKind)
	}

	clusterConfigurationString, ok := config[kubeadmconstants.ClusterConfigurationKind]
	if !ok {
		return nil, fmt.Errorf("%s is not in the map", kubeadmconstants.ClusterConfigurationKind)
	}

	initConfiguration := kubeadmapi.InitConfiguration{}
	if err := json.Unmarshal([]byte(initConfigurationString), &initConfiguration); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(clusterConfigurationString), &initConfiguration.ClusterConfiguration); err != nil {
		return nil, err
	}

	return &Configuration{InitConfiguration: initConfiguration}, nil
}

func getJSONStringFromStruct(i interface{}) (string, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
