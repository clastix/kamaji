// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
	"github.com/clastix/kamaji/internal/sql"
)

type GroupResourceBuilderConfiguration struct {
	client              client.Client
	log                 logr.Logger
	tcpReconcilerConfig TenantControlPlaneReconcilerConfig
	tenantControlPlane  kamajiv1alpha1.TenantControlPlane
	DBConnection        sql.DBConnection
	DataStore           kamajiv1alpha1.DataStore
}

type GroupDeleteableResourceBuilderConfiguration struct {
	client              client.Client
	log                 logr.Logger
	tcpReconcilerConfig TenantControlPlaneReconcilerConfig
	tenantControlPlane  kamajiv1alpha1.TenantControlPlane
	DBConnection        sql.DBConnection
}

// GetResources returns a list of resources that will be used to provide tenant control planes
// Currently there is only a default approach
// TODO: the idea of this function is to become a factory to return the group of resources according to the given configuration.
func GetResources(config GroupResourceBuilderConfiguration, dataStore kamajiv1alpha1.DataStore) []resources.Resource {
	return getDefaultResources(config, dataStore)
}

// GetDeletableResources returns a list of resources that have to be deleted when tenant control planes are deleted
// Currently there is only a default approach
// TODO: the idea of this function is to become a factory to return the group of deleteable resources according to the given configuration.
func GetDeletableResources(config GroupDeleteableResourceBuilderConfiguration, dataStore kamajiv1alpha1.DataStore) []resources.DeleteableResource {
	return getDefaultDeleteableResources(config, dataStore)
}

func getDefaultResources(config GroupResourceBuilderConfiguration, dataStore kamajiv1alpha1.DataStore) []resources.Resource {
	resources := append(getUpgradeResources(config.client, config.tenantControlPlane), getKubernetesServiceResources(config.client, config.tenantControlPlane)...)
	resources = append(resources, getKubeadmConfigResources(config.client, getTmpDirectory(config.tcpReconcilerConfig.TmpBaseDirectory, config.tenantControlPlane), dataStore)...)
	resources = append(resources, getKubernetesCertificatesResources(config.client, config.log, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubeconfigResources(config.client, config.log, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubernetesStorageResources(config.client, config.log, config.tcpReconcilerConfig, config.DBConnection, config.tenantControlPlane, dataStore)...)
	resources = append(resources, getInternalKonnectivityResources(config.client, config.log, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubernetesDeploymentResources(config.client, config.tcpReconcilerConfig, dataStore)...)
	resources = append(resources, getKubernetesIngressResources(config.client, config.tenantControlPlane)...)
	resources = append(resources, getKubeadmPhaseResources(config.client, config.log, config.tenantControlPlane)...)
	resources = append(resources, getKubeadmAddonResources(config.client, config.log, config.tenantControlPlane)...)
	resources = append(resources, getExternalKonnectivityResources(config.client, config.log, config.tcpReconcilerConfig, config.tenantControlPlane)...)

	return resources
}

func getDefaultDeleteableResources(config GroupDeleteableResourceBuilderConfiguration, dataStore kamajiv1alpha1.DataStore) []resources.DeleteableResource {
	switch dataStore.Spec.Driver {
	case kamajiv1alpha1.EtcdDriver:
		return []resources.DeleteableResource{
			&resources.ETCDSetupResource{
				Name:      "etcd-setup",
				Client:    config.client,
				Log:       config.log,
				DataStore: dataStore,
			},
		}
	case kamajiv1alpha1.KineMySQLDriver, kamajiv1alpha1.KinePostgreSQLDriver:
		return []resources.DeleteableResource{
			&resources.SQLSetup{
				Client:       config.client,
				Name:         "sql-setup",
				DBConnection: config.DBConnection,
			},
		}
	default:
		return []resources.DeleteableResource{}
	}
}

func getUpgradeResources(c client.Client, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesUpgrade{
			Name:   "upgrade",
			Client: c,
		},
	}
}

func getKubernetesServiceResources(c client.Client, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesServiceResource{
			Client: c,
		},
	}
}

func getKubeadmConfigResources(c client.Client, tmpDirectory string, dataStore kamajiv1alpha1.DataStore) []resources.Resource {
	var endpoints []string

	switch dataStore.Spec.Driver {
	case kamajiv1alpha1.EtcdDriver:
		endpoints = dataStore.Spec.Endpoints
	default:
		endpoints = []string{"127.0.0.1:2379"}
	}

	return []resources.Resource{
		&resources.KubeadmConfigResource{
			Name:         "kubeadmconfig",
			ETCDs:        endpoints,
			Client:       c,
			TmpDirectory: tmpDirectory,
		},
	}
}

func getKubernetesCertificatesResources(c client.Client, log logr.Logger, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.CACertificate{
			Name:         "ca",
			Client:       c,
			Log:          log,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.FrontProxyCACertificate{
			Name:         "front-proxy-ca-certificate",
			Client:       c,
			Log:          log,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.SACertificate{
			Name:         "sa-certificate",
			Client:       c,
			Log:          log,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.APIServerCertificate{
			Name:         "api-server-certificate",
			Client:       c,
			Log:          log,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.APIServerKubeletClientCertificate{
			Name:         "api-server-kubelet-client-certificate",
			Client:       c,
			Log:          log,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.FrontProxyClientCertificate{
			Name:         "front-proxy-client-certificate",
			Client:       c,
			Log:          log,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
	}
}

func getKubeconfigResources(c client.Client, log logr.Logger, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubeconfigResource{
			Name:               "admin-kubeconfig",
			Client:             c,
			Log:                log,
			KubeConfigFileName: resources.AdminKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "controller-manager-kubeconfig",
			Client:             c,
			Log:                log,
			KubeConfigFileName: resources.ControllerManagerKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "scheduler-kubeconfig",
			Client:             c,
			Log:                log,
			KubeConfigFileName: resources.SchedulerKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
	}
}

func getKubernetesStorageResources(c client.Client, log logr.Logger, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, dbConnection sql.DBConnection, tenantControlPlane kamajiv1alpha1.TenantControlPlane, ds kamajiv1alpha1.DataStore) []resources.Resource {
	switch ds.Spec.Driver {
	case kamajiv1alpha1.EtcdDriver:
		return []resources.Resource{
			&resources.ETCDCACertificatesResource{
				Name:      "etcd-ca-certificates",
				Client:    c,
				Log:       log,
				DataStore: ds,
			},
			&resources.ETCDCertificatesResource{
				Name:   "etcd-certificates",
				Client: c,
				Log:    log,
			},
			&resources.ETCDSetupResource{
				Name:      "etcd-setup",
				Client:    c,
				Log:       log,
				DataStore: ds,
			},
		}
	case kamajiv1alpha1.KineMySQLDriver, kamajiv1alpha1.KinePostgreSQLDriver:
		return []resources.Resource{
			&resources.SQLStorageConfig{
				Client: c,
				Name:   "sql-config",
				Host:   dbConnection.GetHost(),
				Port:   dbConnection.GetPort(),
				Driver: dbConnection.Driver(),
			},
			&resources.SQLSetup{
				Client:       c,
				Name:         "sql-setup",
				DBConnection: dbConnection,
				Driver:       dbConnection.Driver(),
			},
			&resources.SQLCertificate{
				Client:    c,
				Name:      "sql-certificate",
				DataStore: ds,
			},
		}
	default:
		return []resources.Resource{}
	}
}

func getKubernetesDeploymentResources(c client.Client, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, dataStore kamajiv1alpha1.DataStore) []resources.Resource {
	var endpoints []string

	switch dataStore.Spec.Driver {
	case kamajiv1alpha1.EtcdDriver:
		endpoints = dataStore.Spec.Endpoints
	default:
		endpoints = []string{"127.0.0.1:2379"}
	}

	return []resources.Resource{
		&resources.KubernetesDeploymentResource{
			Client:             c,
			ETCDEndpoints:      endpoints,
			DataStoreDriver:    dataStore.Spec.Driver,
			KineContainerImage: tcpReconcilerConfig.KineContainerImage,
		},
	}
}

func getKubernetesIngressResources(c client.Client, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesIngressResource{
			Client: c,
		},
	}
}

func getKubeadmPhaseResources(c client.Client, log logr.Logger, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubeadmPhase{
			Name:   "upload-config-kubeadm",
			Client: c,
			Log:    log,
			Phase:  resources.PhaseUploadConfigKubeadm,
		},
		&resources.KubeadmPhase{
			Name:   "upload-config-kubelet",
			Client: c,
			Log:    log,
			Phase:  resources.PhaseUploadConfigKubelet,
		},
		&resources.KubeadmPhase{
			Name:   "bootstrap-token",
			Client: c,
			Log:    log,
			Phase:  resources.PhaseBootstrapToken,
		},
	}
}

func getKubeadmAddonResources(c client.Client, log logr.Logger, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubeadmAddonResource{
			Name:         "coredns",
			Client:       c,
			Log:          log,
			KubeadmAddon: resources.AddonCoreDNS,
		},
		&resources.KubeadmAddonResource{
			Name:         "kubeproxy",
			Client:       c,
			Log:          log,
			KubeadmAddon: resources.AddonKubeProxy,
		},
	}
}

func getExternalKonnectivityResources(c client.Client, log logr.Logger, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&konnectivity.ServiceAccountResource{
			Client: c,
			Name:   "konnectivity-sa",
		},
		&konnectivity.ClusterRoleBindingResource{
			Client: c,
			Name:   "konnectivity-clusterrolebinding",
		},
		&konnectivity.KubernetesDeploymentResource{
			Client: c,
			Name:   "konnectivity-deployment",
		},
		&konnectivity.ServiceResource{
			Client: c,
			Name:   "konnectivity-service",
		},
		&konnectivity.Agent{
			Client: c,
			Name:   "konnectivity-agent",
		},
	}
}

func getInternalKonnectivityResources(c client.Client, log logr.Logger, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&konnectivity.EgressSelectorConfigurationResource{
			Client: c,
			Name:   "konnectivity-egress-selector-configuration",
		},
		&konnectivity.CertificateResource{
			Client: c,
			Log:    log,
			Name:   "konnectivity-certificate",
		},
		&konnectivity.KubeconfigResource{
			Client: c,
			Name:   "konnectivity-kubeconfig",
		},
	}
}

func getNamespacedName(namespace string, name string) k8stypes.NamespacedName {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}
}

func getTmpDirectory(base string, tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s/%s/%s", base, tenantControlPlane.GetName(), uuid.New())
}
