// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
	builder "github.com/clastix/kamaji/internal/builders/controlplane"
	"github.com/clastix/kamaji/internal/datastore"
	"github.com/clastix/kamaji/internal/resources"
	ds "github.com/clastix/kamaji/internal/resources/datastore"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
	"github.com/clastix/kamaji/internal/utilities"
)

type GroupResourceBuilderConfiguration struct {
	client               client.Client
	log                  logr.Logger
	tcpReconcilerConfig  TenantControlPlaneReconcilerConfig
	tenantControlPlane   kamajiv1alpha1.TenantControlPlane
	ExpirationThreshold  time.Duration
	Connection           datastore.Connection
	DataStore            kamajiv1alpha1.DataStore
	KamajiNamespace      string
	KamajiServiceAccount string
	KamajiService        string
	KamajiMigrateImage   string
	DiscoveryClient      discovery.DiscoveryInterface
}

type GroupDeletableResourceBuilderConfiguration struct {
	client              client.Client
	log                 logr.Logger
	tcpReconcilerConfig TenantControlPlaneReconcilerConfig
	tenantControlPlane  kamajiv1alpha1.TenantControlPlane
	connection          datastore.Connection
	dataStore           kamajiv1alpha1.DataStore
}

// GetResources returns a list of resources that will be used to provide tenant control planes
// Currently there is only a default approach
// TODO: the idea of this function is to become a factory to return the group of resources according to the given configuration.
func GetResources(ctx context.Context, config GroupResourceBuilderConfiguration) []resources.Resource {
	resources := []resources.Resource{}

	resources = append(resources, getDataStoreMigratingResources(config.client, config.KamajiNamespace, config.KamajiMigrateImage, config.KamajiServiceAccount, config.KamajiService)...)
	resources = append(resources, getUpgradeResources(config.client)...)
	resources = append(resources, getKubernetesServiceResources(config.client)...)
	resources = append(resources, getKubeadmConfigResources(config.client, getTmpDirectory(config.tcpReconcilerConfig.TmpBaseDirectory, config.tenantControlPlane), config.DataStore)...)
	resources = append(resources, getKubernetesCertificatesResources(config.client, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubeconfigResources(config.client, config.tcpReconcilerConfig, config.tenantControlPlane)...)
	resources = append(resources, getKubernetesStorageResources(config.client, config.Connection, config.DataStore, config.ExpirationThreshold)...)
	resources = append(resources, getKonnectivityServerRequirementsResources(config.client, config.ExpirationThreshold)...)
	resources = append(resources, getKubernetesDeploymentResources(config.client, config.tcpReconcilerConfig, config.DataStore)...)
	resources = append(resources, getKonnectivityServerPatchResources(config.client)...)
	resources = append(resources, getDataStoreMigratingCleanup(config.client, config.KamajiNamespace)...)
	resources = append(resources, getKubernetesIngressResources(config.client)...)

	// Conditionally add Gateway resources
	if utilities.AreGatewayResourcesAvailable(ctx, config.client, config.DiscoveryClient) {
		resources = append(resources, getKubernetesGatewayResources(config.client)...)
	}

	return resources
}

// GetDeletableResources returns a list of resources that have to be deleted when tenant control planes are deleted
// Currently there is only a default approach
// TODO: the idea of this function is to become a factory to return the group of deletable resources according to the given configuration.
func GetDeletableResources(tcp *kamajiv1alpha1.TenantControlPlane, config GroupDeletableResourceBuilderConfiguration) []resources.DeletableResource {
	var res []resources.DeletableResource

	if controllerutil.ContainsFinalizer(tcp, finalizers.DatastoreFinalizer) {
		res = append(res, &ds.Setup{
			Client:     config.client,
			Connection: config.connection,
		})
		res = append(res, &ds.Config{
			Client:     config.client,
			ConnString: config.connection.GetConnectionString(),
			DataStore:  config.dataStore,
		})
	}

	return res
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

func getKubernetesGatewayResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&resources.KubernetesGatewayResource{
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
			Client:                  c,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.FrontProxyCACertificate{
			Client:                  c,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.SACertificate{
			Client:       c,
			TmpDirectory: getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
		},
		&resources.APIServerCertificate{
			Client:                  c,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.APIServerKubeletClientCertificate{
			Client:                  c,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.FrontProxyClientCertificate{
			Client:                  c,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
	}
}

func getKubeconfigResources(c client.Client, tcpReconcilerConfig TenantControlPlaneReconcilerConfig, tenantControlPlane kamajiv1alpha1.TenantControlPlane) []resources.Resource {
	return []resources.Resource{
		&resources.KubeconfigResource{
			Client:                  c,
			Name:                    "admin-kubeconfig",
			KubeConfigFileName:      resources.AdminKubeConfigFileName,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.KubeconfigResource{
			Client:                  c,
			Name:                    "admin-kubeconfig",
			KubeConfigFileName:      resources.SuperAdminKubeConfigFileName,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.KubeconfigResource{
			Client:                  c,
			Name:                    "controller-manager-kubeconfig",
			KubeConfigFileName:      resources.ControllerManagerKubeConfigFileName,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
		&resources.KubeconfigResource{
			Client:                  c,
			Name:                    "scheduler-kubeconfig",
			KubeConfigFileName:      resources.SchedulerKubeConfigFileName,
			TmpDirectory:            getTmpDirectory(tcpReconcilerConfig.TmpBaseDirectory, tenantControlPlane),
			CertExpirationThreshold: tcpReconcilerConfig.CertExpirationThreshold,
		},
	}
}

func getKubernetesStorageResources(c client.Client, dbConnection datastore.Connection, datastore kamajiv1alpha1.DataStore, threshold time.Duration) []resources.Resource {
	return []resources.Resource{
		&ds.MultiTenancy{
			DataStore: datastore,
		},
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
			Client:                  c,
			DataStore:               datastore,
			CertExpirationThreshold: threshold,
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

func GetExternalKonnectivityResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&konnectivity.Agent{Client: c},
		&konnectivity.ServiceAccountResource{Client: c},
		&konnectivity.ClusterRoleBindingResource{Client: c},
	}
}

func getKonnectivityServerRequirementsResources(c client.Client, threshold time.Duration) []resources.Resource {
	return []resources.Resource{
		&konnectivity.EgressSelectorConfigurationResource{Client: c},
		&konnectivity.CertificateResource{Client: c, CertExpirationThreshold: threshold},
		&konnectivity.KubeconfigResource{Client: c},
	}
}

func getKonnectivityServerPatchResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&konnectivity.KubernetesDeploymentResource{Builder: builder.Konnectivity{Scheme: *c.Scheme()}, Client: c},
		&konnectivity.ServiceResource{Client: c},
	}
}

func getNamespacedName(namespace string, name string) k8stypes.NamespacedName {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}
}

func getTmpDirectory(base string, tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s/%s/%s", base, tenantControlPlane.GetName(), uuid.New())
}
