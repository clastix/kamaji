// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// APIServerCertificatesStatus defines the observed state of ETCD Certificate for API server.
type APIServerCertificatesStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// ETCDCertificateStatus defines the observed state of ETCD Certificate for API server.
type ETCDCertificateStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// ETCDCertificatesStatus defines the observed state of ETCD Certificate for API server.
type ETCDCertificatesStatus struct {
	APIServer APIServerCertificatesStatus `json:"apiServer,omitempty"`
	CA        ETCDCertificateStatus       `json:"ca,omitempty"`
}

// CertificatePrivateKeyPairStatus defines the status.
type CertificatePrivateKeyPairStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// PublicKeyPrivateKeyPairStatus defines the status.
type PublicKeyPrivateKeyPairStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// CertificatesStatus defines the observed state of ETCD TLSConfig.
type CertificatesStatus struct {
	CA                     CertificatePrivateKeyPairStatus `json:"ca,omitempty"`
	APIServer              CertificatePrivateKeyPairStatus `json:"apiServer,omitempty"`
	APIServerKubeletClient CertificatePrivateKeyPairStatus `json:"apiServerKubeletClient,omitempty"`
	FrontProxyCA           CertificatePrivateKeyPairStatus `json:"frontProxyCA,omitempty"`
	FrontProxyClient       CertificatePrivateKeyPairStatus `json:"frontProxyClient,omitempty"`
	SA                     PublicKeyPrivateKeyPairStatus   `json:"sa,omitempty"`
	ETCD                   *ETCDCertificatesStatus         `json:"etcd,omitempty"`
}

type DataStoreCertificateStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

type DataStoreConfigStatus struct {
	SecretName string `json:"secretName,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

type DataStoreSetupStatus struct {
	Schema     string      `json:"schema,omitempty"`
	User       string      `json:"user,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// StorageStatus defines the observed state of StorageStatus.
type StorageStatus struct {
	Driver        string                     `json:"driver,omitempty"`
	DataStoreName string                     `json:"dataStoreName,omitempty"`
	Config        DataStoreConfigStatus      `json:"config,omitempty"`
	Setup         DataStoreSetupStatus       `json:"setup,omitempty"`
	Certificate   DataStoreCertificateStatus `json:"certificate,omitempty"`
}

// KubeconfigStatus contains information about the generated kubeconfig.
type KubeconfigStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// KubeconfigsStatus stores information about all the generated kubeconfig resources.
type KubeconfigsStatus struct {
	Admin             KubeconfigStatus `json:"admin,omitempty"`
	ControllerManager KubeconfigStatus `json:"controllerManager,omitempty"`
	Scheduler         KubeconfigStatus `json:"scheduler,omitempty"`
}

// KubeadmConfigStatus contains the status of the configuration required by kubeadm.
type KubeadmConfigStatus struct {
	ConfigmapName string      `json:"configmapName,omitempty"`
	LastUpdate    metav1.Time `json:"lastUpdate,omitempty"`
	// Checksum of the kubeadm configuration to detect changes
	Checksum string `json:"checksum,omitempty"`
}

// KubeadmPhaseStatus contains the status of a kubeadm phase action.
type KubeadmPhaseStatus struct {
	Checksum   string      `json:"checksum,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// KubeadmPhasesStatus contains the status of the different kubeadm phases action.
type KubeadmPhasesStatus struct {
	BootstrapToken KubeadmPhaseStatus `json:"bootstrapToken"`
}

type ExternalKubernetesObjectStatus struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	// Last time when k8s object was updated
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// KonnectivityStatus defines the status of Konnectivity as Addon.
type KonnectivityStatus struct {
	Enabled            bool                            `json:"enabled"`
	ConfigMap          KonnectivityConfigMap           `json:"configMap,omitempty"`
	Certificate        CertificatePrivateKeyPairStatus `json:"certificate,omitempty"`
	Kubeconfig         KubeconfigStatus                `json:"kubeconfig,omitempty"`
	ServiceAccount     ExternalKubernetesObjectStatus  `json:"sa,omitempty"`
	ClusterRoleBinding ExternalKubernetesObjectStatus  `json:"clusterrolebinding,omitempty"`
	Agent              ExternalKubernetesObjectStatus  `json:"agent,omitempty"`
	Service            KubernetesServiceStatus         `json:"service,omitempty"`
}

type KonnectivityConfigMap struct {
	Name     string `json:"name,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// AddonStatus defines the observed state of an Addon.
type AddonStatus struct {
	Enabled    bool        `json:"enabled"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// AddonsStatus defines the observed state of the different Addons.
type AddonsStatus struct {
	CoreDNS      AddonStatus        `json:"coreDNS,omitempty"`
	KubeProxy    AddonStatus        `json:"kubeProxy,omitempty"`
	Konnectivity KonnectivityStatus `json:"konnectivity,omitempty"`
}

// TenantControlPlaneStatus defines the observed state of TenantControlPlane.
type TenantControlPlaneStatus struct {
	// Storage Status contains information about Kubernetes storage system
	Storage StorageStatus `json:"storage,omitempty"`
	// Certificates contains information about the different certificates
	// that are necessary to run a kubernetes control plane
	Certificates CertificatesStatus `json:"certificates,omitempty"`
	// KubeConfig contains information about the kubenconfigs that control plane pieces need
	KubeConfig KubeconfigsStatus `json:"kubeconfig,omitempty"`
	// Kubernetes contains information about the reconciliation of the required Kubernetes resources deployed in the admin cluster
	Kubernetes KubernetesStatus `json:"kubernetesResources,omitempty"`
	// KubeadmConfig contains the status of the configuration required by kubeadm
	KubeadmConfig KubeadmConfigStatus `json:"kubeadmconfig,omitempty"`
	// KubeadmPhase contains the status of the kubeadm phases action
	KubeadmPhase KubeadmPhasesStatus `json:"kubeadmPhase,omitempty"`
	// ControlPlaneEndpoint contains the status of the kubernetes control plane
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint,omitempty"`
	// Addons contains the status of the different Addons
	Addons AddonsStatus `json:"addons,omitempty"`
}

// KubernetesStatus defines the status of the resources deployed in the management cluster,
// such as Deployment and Service.
type KubernetesStatus struct {
	// KubernetesVersion contains the information regarding the running Kubernetes version, and its upgrade status.
	Version    KubernetesVersion          `json:"version,omitempty"`
	Deployment KubernetesDeploymentStatus `json:"deployment,omitempty"`
	Service    KubernetesServiceStatus    `json:"service,omitempty"`
	Ingress    *KubernetesIngressStatus   `json:"ingress,omitempty"`
}

// +kubebuilder:validation:Enum=Provisioning;CertificateAuthorityRotating;Upgrading;Migrating;Ready;NotReady
type KubernetesVersionStatus string

var (
	VersionProvisioning KubernetesVersionStatus = "Provisioning"
	VersionCARotating   KubernetesVersionStatus = "CertificateAuthorityRotating"
	VersionUpgrading    KubernetesVersionStatus = "Upgrading"
	VersionMigrating    KubernetesVersionStatus = "Migrating"
	VersionReady        KubernetesVersionStatus = "Ready"
	VersionNotReady     KubernetesVersionStatus = "NotReady"
)

type KubernetesVersion struct {
	// Version is the running Kubernetes version of the Tenant Control Plane.
	Version string `json:"version,omitempty"`
	// +kubebuilder:default=Provisioning
	// Status returns the current status of the Kubernetes version, such as its provisioning state, or completed upgrade.
	Status *KubernetesVersionStatus `json:"status,omitempty"`
}

// KubernetesDeploymentStatus defines the status for the Tenant Control Plane Deployment in the management cluster.
type KubernetesDeploymentStatus struct {
	appsv1.DeploymentStatus `json:",inline"`
	// Selector is the label selector used to group the Tenant Control Plane Pods used by the scale subresource.
	Selector string `json:"selector"`
	// The name of the Deployment for the given cluster.
	Name string `json:"name"`
	// The namespace which the Deployment for the given cluster is deployed.
	Namespace string `json:"namespace"`
	// Last time when deployment was updated
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// KubernetesServiceStatus defines the status for the Tenant Control Plane Service in the management cluster.
type KubernetesServiceStatus struct {
	corev1.ServiceStatus `json:",inline"`
	// The name of the Service for the given cluster.
	Name string `json:"name"`
	// The namespace which the Service for the given cluster is deployed.
	Namespace string `json:"namespace"`
	// The port where the service is running
	Port int32 `json:"port"`
}

// KubernetesIngressStatus defines the status for the Tenant Control Plane Ingress in the management cluster.
type KubernetesIngressStatus struct {
	networkingv1.IngressStatus `json:",inline"`
	// The name of the Ingress for the given cluster.
	Name string `json:"name"`
	// The namespace which the Ingress for the given cluster is deployed.
	Namespace string `json:"namespace"`
}
