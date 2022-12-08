// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
	"github.com/clastix/kamaji/internal/datastore"
	"github.com/clastix/kamaji/internal/resources"
	ds "github.com/clastix/kamaji/internal/resources/datastore"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
)

type GroupResourceBuilderConfiguration struct {
	client               client.Client
	log                  logr.Logger
	tcpReconcilerConfig  TenantControlPlaneReconcilerConfig
	tenantControlPlane   kamajiv1alpha1.TenantControlPlane
	Connection           datastore.Connection
	DataStore            kamajiv1alpha1.DataStore
	KamajiNamespace      string
	KamajiServiceAccount string
	KamajiService        string
	KamajiMigrateImage   string
}

type GroupDeletableResourceBuilderConfiguration struct {
	client              client.Client
	log                 logr.Logger
	tcpReconcilerConfig TenantControlPlaneReconcilerConfig
	tenantControlPlane  kamajiv1alpha1.TenantControlPlane
	connection          datastore.Connection
}

// GetResources returns a list of resources that will be used to provide tenant control planes
// Currently there is only a default approach
// TODO: the idea of this function is to become a factory to return the group of resources according to the given configuration.
func GetResources(config GroupResourceBuilderConfiguration) []resources.Resource {
	return getDefaultResources(config)
}

// GetDeletableResources returns a list of resources that have to be deleted when tenant control planes are deleted
// Currently there is only a default approach
// TODO: the idea of this function is to become a factory to return the group of deletable resources according to the given configuration.
func GetDeletableResources(tcp *kamajiv1alpha1.TenantControlPlane, config GroupDeletableResourceBuilderConfiguration) []resources.DeletableResource {
	var res []resources.DeletableResource

	if controllerutil.ContainsFinalizer(tcp, finalizers.TenantControlPlaneFinalizer) {
		res = append(res, &ds.Setup{
			Client:     config.client,
			Connection: config.connection,
		})
	}

	return res
}

func getDefaultResources(config GroupResourceBuilderConfiguration) []resources.Resource {
	resources := getDataStoreMigratingResources(config.client, config.KamajiNamespace, config.KamajiMigrateImage, config.KamajiServiceAccount, config.KamajiService)
	resources = append(resources, getUpgradeResources(config.client)...)
	resources = append(resources, getKubernetesServiceResources(config.client)...)
	resources = append(resources, getKubeadmConfigResources(config.client, getTmpDirectory(config.tcpReconcilerConfig.TmpBaseDirectory, config.tenantControlPlane), config.DataStore)...)
	resources = append(resources, getKubernetesCertificatesResources(config.client, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubeconfigResources(config.client, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubernetesStorageResources(config.client, config.Connection, config.DataStore)...)
	resources = append(resources, getInternalKonnectivityResources(config.client)...)
	resources = append(resources, getKubernetesDeploymentResources(config.client, config.tcpReconcilerConfig, config.DataStore)...)
	resources = append(resources, getDataStoreMigratingCleanup(config.client, config.KamajiNamespace)...)
	resources = append(resources, getKubernetesIngressResources(config.client)...)
	resources = append(resources, getKubeadmPhaseResources(config.client)...)
	resources = append(resources, getKubeadmAddonResources(config.client)...)
	resources = append(resources, getExternalKonnectivityResources(config.client)...)

	return resources
}

func getDataStoreMigratingCleanup(c client.Client, kamajiNamespace string) []resources.Resource {
	return []resources.Resource{
		&ds.Migrate{
			Client:          c,
			KamajiNamespace: kamajiNamespace,
			ShouldCleanUp:   true,
		},
	}
}

func getDataStoreMigratingResources(c client.Client, kamajiNamespace, migrateImage string, kamajiServiceAccount, kamajiService string) []resources.Resource {
	return []resources.Resource{
		&ds.Migrate{
			Client:               c,
			MigrateImage:         migrateImage,
			KamajiNamespace:      kamajiNamespace,
			KamajiServiceAccount: kamajiServiceAccount,
			KamajiServiceName:    kamajiService,
		},
	}
}

func getUpgradeResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesUpgrade{
			Client: c,
		},
	}
}

func getKubernetesServiceResources(c client.Client) []resources.Resource {
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
			ETCDs:        endpoints,
			Client:       c,
			TmpDirectory: tmpDirectory,
		},
	}
}

func getKubernetesCertificatesResources(c client.Client, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.CACertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.FrontProxyCACertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.SACertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.APIServerCertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.APIServerKubeletClientCertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.FrontProxyClientCertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
	}
}

func getKubeconfigResources(c client.Client, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubeconfigResource{
			Name:               "admin-kubeconfig",
			Client:             c,
			KubeConfigFileName: resources.AdminKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "controller-manager-kubeconfig",
			Client:             c,
			KubeConfigFileName: resources.ControllerManagerKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "scheduler-kubeconfig",
			Client:             c,
			KubeConfigFileName: resources.SchedulerKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
	}
}

func getKubernetesStorageResources(c client.Client, dbConnection datastore.Connection, datastore kamajiv1alpha1.DataStore) []resources.Resource {
	return []resources.Resource{
		&ds.Config{
			Client:     c,
			ConnString: dbConnection.GetConnectionString(),
			DataStore:  datastore,
		},
		&ds.Setup{
			Client:     c,
			Connection: dbConnection,
			DataStore:  datastore,
		},
		&ds.Certificate{
			Client:    c,
			DataStore: datastore,
		},
	}
}

func getKubernetesDeploymentResources(c client.Client, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, dataStore kamajiv1alpha1.DataStore) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesDeploymentResource{
			Client:             c,
			DataStore:          dataStore,
			KineContainerImage: tcpReconcilerConfig.KineContainerImage,
		},
	}
}

func getKubernetesIngressResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesIngressResource{
			Client: c,
		},
	}
}

func getKubeadmPhaseResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&resources.KubeadmPhase{
			Name:   "upload-config-kubeadm",
			Client: c,
			Phase:  resources.PhaseUploadConfigKubeadm,
		},
		&resources.KubeadmPhase{
			Name:   "upload-config-kubelet",
			Client: c,
			Phase:  resources.PhaseUploadConfigKubelet,
		},
		&resources.KubeadmPhase{
			Name:   "bootstrap-token",
			Client: c,
			Phase:  resources.PhaseBootstrapToken,
		},
	}
}

func getKubeadmAddonResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&resources.KubeadmAddonResource{
			Name:         "coredns",
			Client:       c,
			KubeadmAddon: resources.AddonCoreDNS,
		},
		&resources.KubeadmAddonResource{
			Name:         "kubeproxy",
			Client:       c,
			KubeadmAddon: resources.AddonKubeProxy,
		},
	}
}

func getExternalKonnectivityResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&konnectivity.ServiceAccountResource{Client: c},
		&konnectivity.ClusterRoleBindingResource{Client: c},
		&konnectivity.KubernetesDeploymentResource{Client: c},
		&konnectivity.ServiceResource{Client: c},
		&konnectivity.Agent{Client: c},
	}
}

func getInternalKonnectivityResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&konnectivity.EgressSelectorConfigurationResource{Client: c},
		&konnectivity.CertificateResource{Client: c},
		&konnectivity.KubeconfigResource{Client: c},
	}
}

func getNamespacedName(namespace string, name string) k8stypes.NamespacedName {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}
}

func getTmpDirectory(base string, tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s/%s/%s", base, tenantControlPlane.GetName(), uuid.New())
}
