// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clastix/kamaji/internal/etcd"
)

// NetworkProfileSpec defines the desired state of NetworkProfile.
type NetworkProfileSpec struct {
	// Address where API server of will be exposed.
	// In case of LoadBalancer Service, this can be empty in order to use the exposed IP provided by the cloud controller manager.
	Address string `json:"address,omitempty"`
	// AllowAddressAsExternalIP will include tenantControlPlane.Spec.NetworkProfile.Address in the section of
	// ExternalIPs of the Kubernetes Service (only ClusterIP or NodePort)
	AllowAddressAsExternalIP bool `json:"allowAddressAsExternalIP,omitempty"`
	// Port where API server of will be exposed
	// +kubebuilder:default=6443
	Port int32 `json:"port,omitempty"`
	// CertSANs sets extra Subject Alternative Names (SANs) for the API Server signing certificate.
	// Use this field to add additional hostnames when exposing the Tenant Control Plane with third solutions.
	CertSANs []string `json:"certSANs,omitempty"`
	// Kubernetes Service
	// +kubebuilder:default="10.96.0.0/16"
	ServiceCIDR string `json:"serviceCidr,omitempty"`
	// CIDR for Kubernetes Pods
	// +kubebuilder:default="10.244.0.0/16"
	PodCIDR string `json:"podCidr,omitempty"`
	// +kubebuilder:default={"10.96.0.10"}
	DNSServiceIPs []string `json:"dnsServiceIPs,omitempty"`
}

type KubeletSpec struct {
	// CGroupFS defines the  cgroup driver for Kubelet
	// https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/configure-cgroup-driver/
	CGroupFS CGroupDriver `json:"cgroupfs,omitempty"`
}

// KubernetesSpec defines the desired state of Kubernetes.
type KubernetesSpec struct {
	// Kubernetes Version for the tenant control plane
	Version string      `json:"version"`
	Kubelet KubeletSpec `json:"kubelet"`

	// List of enabled Admission Controllers for the Tenant cluster.
	// Full reference available here: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers
	// +kubebuilder:default=CertificateApproval;CertificateSigning;CertificateSubjectRestriction;DefaultIngressClass;DefaultStorageClass;DefaultTolerationSeconds;LimitRanger;MutatingAdmissionWebhook;NamespaceLifecycle;PersistentVolumeClaimResize;Priority;ResourceQuota;RuntimeClass;ServiceAccount;StorageObjectInUseProtection;TaintNodesByCondition;ValidatingAdmissionWebhook
	AdmissionControllers AdmissionControllers `json:"admissionControllers,omitempty"`
}

// AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.
type AdditionalMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ControlPlane defines how the Tenant Control Plane Kubernetes resources must be created in the Admin Cluster,
// such as the number of Pod replicas, the Service resource, or the Ingress.
type ControlPlane struct {
	// Defining the options for the deployed Tenant Control Plane as Deployment resource.
	Deployment DeploymentSpec `json:"deployment,omitempty"`
	// Defining the options for the Tenant Control Plane Service resource.
	Service ServiceSpec `json:"service"`
	// Defining the options for an Optional Ingress which will expose API Server of the Tenant Control Plane
	Ingress IngressSpec `json:"ingress,omitempty"`
}

// IngressSpec defines the options for the ingress which will expose API Server of the Tenant Control Plane.
type IngressSpec struct {
	AdditionalMetadata AdditionalMetadata `json:"additionalMetadata,omitempty"`
	Enabled            bool               `json:"enabled"`
	IngressClassName   string             `json:"ingressClassName,omitempty"`
	// Hostname is an optional field which will be used as Ingress's Host. If it is not defined,
	// Ingress's host will be "<tenant>.<namespace>.<domain>", where domain is specified under NetworkProfileSpec
	Hostname string `json:"hostname,omitempty"`
}

type DeploymentSpec struct {
	// +kubebuilder:default=2
	Replicas           int32              `json:"replicas,omitempty"`
	AdditionalMetadata AdditionalMetadata `json:"additionalMetadata,omitempty"`
}

type ServiceSpec struct {
	AdditionalMetadata AdditionalMetadata `json:"additionalMetadata,omitempty"`
	// ServiceType allows specifying how to expose the Tenant Control Plane.
	ServiceType ServiceType `json:"serviceType"`
}

// AddonSpec defines the spec for every addon.
type AddonSpec struct{}

// KonnectivitySpec defines the spec for Konnectivity.
type KonnectivitySpec struct {
	// Port of Konnectivity proxy server.
	ProxyPort int32 `json:"proxyPort"`
	// Host of Konnectivity proxy server.
	ProxyHost                string `json:"proxyHost,omitempty"`
	AllowAddressAsExternalIP bool   `json:"allowAddressAsExternalIP,omitempty"`
	// ServiceType allows specifying how to expose the Konnectivity Proxy Server.
	ServiceType ServiceType `json:"serviceType"`
	// Version for Konnectivity server and agent.
	// +kubebuilder:default=v0.0.31
	Version string `json:"version,omitempty"`
	// ServerImage defines the container image for Konnectivity's server.
	// +kubebuilder:default=us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-server
	ServerImage string `json:"serverImage,omitempty"`
	// AgentImage defines the container image for Konnectivity's agent.
	// +kubebuilder:default=us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent
	AgentImage string `json:"agentImage,omitempty"`
}

// AddonsSpec defines the enabled addons and their features.
type AddonsSpec struct {
	CoreDNS      *AddonSpec        `json:"coreDNS,omitempty"`
	Konnectivity *KonnectivitySpec `json:"konnectivity,omitempty"`
	KubeProxy    *AddonSpec        `json:"kubeProxy,omitempty"`
}

// TenantControlPlaneSpec defines the desired state of TenantControlPlane.
type TenantControlPlaneSpec struct {
	ControlPlane ControlPlane `json:"controlPlane"`

	// Kubernetes specification for tenant control plane
	Kubernetes KubernetesSpec `json:"kubernetes"`

	// NetworkProfile specifies how the network is
	NetworkProfile NetworkProfileSpec `json:"networkProfile,omitempty"`

	// Addons contain which addons are enabled
	Addons AddonsSpec `json:"addons,omitempty"`
}

// ETCDAPIServerCertificate defines the observed state of ETCD Certificate for API server.
type APIServerCertificatesStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// ETCDAPIServerCertificate defines the observed state of ETCD Certificate for API server.
type ETCDCertificateStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// ETCDAPIServerCertificate defines the observed state of ETCD Certificate for API server.
type ETCDCertificatesStatus struct {
	APIServer ETCDCertificateStatus `json:"apiServer,omitempty"`
	CA        ETCDCertificateStatus `json:"ca,omitempty"`
}

// CertificatePrivateKeyPair defines the status.
type CertificatePrivateKeyPairStatus struct {
	SecretName      string      `json:"secretName,omitempty"`
	LastUpdate      metav1.Time `json:"lastUpdate,omitempty"`
	ResourceVersion string      `json:"resourceVersion,omitempty"`
}

// CertificatePrivateKeyPair defines the status.
type PublicKeyPrivateKeyPairStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// ETCDCertificates defines the observed state of ETCD Certificates.
type CertificatesStatus struct {
	CA                     CertificatePrivateKeyPairStatus `json:"ca,omitempty"`
	APIServer              CertificatePrivateKeyPairStatus `json:"apiServer,omitempty"`
	APIServerKubeletClient CertificatePrivateKeyPairStatus `json:"apiServerKubeletClient,omitempty"`
	FrontProxyCA           CertificatePrivateKeyPairStatus `json:"frontProxyCA,omitempty"`
	FrontProxyClient       CertificatePrivateKeyPairStatus `json:"frontProxyClient,omitempty"`
	SA                     PublicKeyPrivateKeyPairStatus   `json:"sa,omitempty"`
	ETCD                   *ETCDCertificatesStatus         `json:"etcd,omitempty"`
}

// ETCDStatus defines the observed state of ETCDStatus.
type ETCDStatus struct {
	Role etcd.Role `json:"role,omitempty"`
	User etcd.User `json:"user,omitempty"`
}

type SQLCertificateStatus struct {
	SecretName      string      `json:"secretName,omitempty"`
	ResourceVersion string      `json:"resourceVersion,omitempty"`
	LastUpdate      metav1.Time `json:"lastUpdate,omitempty"`
}

type SQLConfigStatus struct {
	SecretName      string `json:"secretName,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

type SQLSetup struct {
	Schema                   string      `json:"schema,omitempty"`
	User                     string      `json:"user,omitempty"`
	LastUpdate               metav1.Time `json:"lastUpdate,omitempty"`
	SQLConfigResourceVersion string      `json:"sqlConfigResourceVersion,omitempty"`
}

type KineMySQLStatus struct {
	Config      SQLConfigStatus      `json:"config,omitempty"`
	Setup       SQLSetup             `json:"setup,omitempty"`
	Certificate SQLCertificateStatus `json:"certificate,omitempty"`
}

// StorageStatus defines the observed state of StorageStatus.
type StorageStatus struct {
	ETCD      *ETCDStatus      `json:"etcd,omitempty"`
	KineMySQL *KineMySQLStatus `json:"kineMySQL,omitempty"`
}

// KubeconfigStatus contains information about the generated kubeconfig.
type KubeconfigStatus struct {
	SecretName string      `json:"secretName,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	Checksum   string      `json:"checksum,omitempty"`
}

// KubeconfigsStatus stores information about all the generated kubeconfigs.
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

// KubeadmPhasesStatus contains the status of of a kubeadm phase action.
type KubeadmPhaseStatus struct {
	Checksum   string      `json:"checksum,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

func (in KubeadmPhaseStatus) GetChecksum() string {
	return in.Checksum
}

func (in *KubeadmPhaseStatus) SetChecksum(rv string) {
	in.Checksum = rv
}

// KubeadmPhasesStatus contains the status of the different kubeadm phases action.
type KubeadmPhasesStatus struct {
	UploadConfigKubeadm KubeadmPhaseStatus `json:"uploadConfigKubeadm"`
	UploadConfigKubelet KubeadmPhaseStatus `json:"uploadConfigKubelet"`
	BootstrapToken      KubeadmPhaseStatus `json:"bootstrapToken"`
}

type ExternalKubernetesObjectStatus struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	// Resource version of k8s object
	RV string `json:"resourceVersion,omitempty"`
	// Last time when k8s object was updated
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// KonnectivityStatus defines the status of Konnectivity as Addon.
type KonnectivityStatus struct {
	Enabled                     bool                            `json:"enabled"`
	EgressSelectorConfiguration string                          `json:"egressSelectorConfiguration,omitempty"`
	Certificate                 CertificatePrivateKeyPairStatus `json:"certificate,omitempty"`
	Kubeconfig                  KubeconfigStatus                `json:"kubeconfig,omitempty"`
	ServiceAccount              ExternalKubernetesObjectStatus  `json:"sa,omitempty"`
	ClusterRoleBinding          ExternalKubernetesObjectStatus  `json:"clusterrolebinding,omitempty"`
	Agent                       ExternalKubernetesObjectStatus  `json:"agent,omitempty"`
	Service                     KubernetesServiceStatus         `json:"service,omitempty"`
}

// AddonStatus defines the observed state of an Addon.
type AddonStatus struct {
	Enabled    bool        `json:"enabled"`
	Checksum   string      `json:"checksum,omitempty"`
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

func (in AddonStatus) GetChecksum() string {
	return in.Checksum
}

func (in *AddonStatus) SetChecksum(rv string) {
	in.Checksum = rv
}

// AddonsStatus defines the observed state of the different Addons.
type AddonsStatus struct {
	CoreDNS   AddonStatus `json:"coreDNS,omitempty"`
	KubeProxy AddonStatus `json:"kubeProxy,omitempty"`

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
	Ingress    KubernetesIngressStatus    `json:"ingress,omitempty"`
}

// +kubebuilder:validation:Enum=Provisioning;Upgrading;Ready;NotReady
type KubernetesVersionStatus string

var (
	VersionProvisioning KubernetesVersionStatus = "Provisioning"
	VersionUpgrading    KubernetesVersionStatus = "Upgrading"
	VersionReady        KubernetesVersionStatus = "Ready"
	VersionNotReady     KubernetesVersionStatus = "NotReady"
)

type KubernetesVersion struct {
	// Version is the running Kubernetes version of the Tenant Control Plane.
	Version string `json:"version,omitempty"`
	// +kubebuilder:default=Provisioning
	// Status returns the current status of the Kubernetes version, such as its provisioning state, or completed upgrade.
	Status *KubernetesVersionStatus `json:"status"`
}

// KubernetesDeploymentStatus defines the status for the Tenant Control Plane Deployment in the management cluster.
type KubernetesDeploymentStatus struct {
	appv1.DeploymentStatus `json:",inline"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tcp
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetes.version",description="Kubernetes version"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.kubernetesResources.version.status",description="Kubernetes version"
// +kubebuilder:printcolumn:name="Control-Plane-Endpoint",type="string",JSONPath=".status.controlPlaneEndpoint",description="Tenant Control Plane Endpoint (API server)"
// +kubebuilder:printcolumn:name="Kubeconfig",type="string",JSONPath=".status.kubeconfig.admin.secretName",description="Secret which contains admin kubeconfig"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Age"

// TenantControlPlane is the Schema for the tenantcontrolplanes API.
type TenantControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantControlPlaneSpec   `json:"spec,omitempty"`
	Status TenantControlPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantControlPlaneList contains a list of TenantControlPlane.
type TenantControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantControlPlane{}, &TenantControlPlaneList{})
}
