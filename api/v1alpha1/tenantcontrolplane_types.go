// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Ingress *IngressSpec `json:"ingress,omitempty"`
}

// IngressSpec defines the options for the ingress which will expose API Server of the Tenant Control Plane.
type IngressSpec struct {
	AdditionalMetadata AdditionalMetadata `json:"additionalMetadata,omitempty"`
	IngressClassName   string             `json:"ingressClassName,omitempty"`
	// Hostname is an optional field which will be used as Ingress's Host. If it is not defined,
	// Ingress's host will be "<tenant>.<namespace>.<domain>", where domain is specified under NetworkProfileSpec
	Hostname string `json:"hostname,omitempty"`
}

type ControlPlaneComponentsResources struct {
	APIServer         *corev1.ResourceRequirements `json:"apiServer,omitempty"`
	ControllerManager *corev1.ResourceRequirements `json:"controllerManager,omitempty"`
	Scheduler         *corev1.ResourceRequirements `json:"scheduler,omitempty"`
}

type DeploymentSpec struct {
	// +kubebuilder:default=2
	Replicas int32 `json:"replicas,omitempty"`
	// Resources defines the amount of memory and CPU to allocate to each component of the Control Plane
	// (kube-apiserver, controller-manager, and scheduler).
	Resources *ControlPlaneComponentsResources `json:"resources,omitempty"`
	// ExtraArgs allows adding additional arguments to the Control Plane components,
	// such as kube-apiserver, controller-manager, and scheduler.
	ExtraArgs          *ControlPlaneExtraArgs `json:"extraArgs,omitempty"`
	AdditionalMetadata AdditionalMetadata     `json:"additionalMetadata,omitempty"`
}

// ControlPlaneExtraArgs allows specifying additional arguments to the Control Plane components.
type ControlPlaneExtraArgs struct {
	APIServer         []string `json:"apiServer,omitempty"`
	ControllerManager []string `json:"controllerManager,omitempty"`
	Scheduler         []string `json:"scheduler,omitempty"`
	// Available only if Kamaji is running using Kine as backing storage.
	Kine []string `json:"kine,omitempty"`
}

type ServiceSpec struct {
	AdditionalMetadata AdditionalMetadata `json:"additionalMetadata,omitempty"`
	// ServiceType allows specifying how to expose the Tenant Control Plane.
	ServiceType ServiceType `json:"serviceType"`
}

// AddonSpec defines the spec for every addon.
type AddonSpec struct{}

type KubeProxySpec struct {
	// Specify the image overried of the kube-proxy to install in the Tenant Cluster.
	// If not specified, the Kubernetes default one will be used, according to the specified version.
	ImageOverride string `json:"imageOverride,omitempty"`
}

// KonnectivitySpec defines the spec for Konnectivity.
type KonnectivitySpec struct {
	// Port of Konnectivity proxy server.
	ProxyPort int32 `json:"proxyPort"`
	// Version for Konnectivity server and agent.
	// +kubebuilder:default=v0.0.31
	Version string `json:"version,omitempty"`
	// ServerImage defines the container image for Konnectivity's server.
	// +kubebuilder:default=us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-server
	ServerImage string `json:"serverImage,omitempty"`
	// AgentImage defines the container image for Konnectivity's agent.
	// +kubebuilder:default=us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent
	AgentImage string `json:"agentImage,omitempty"`
	// Resources define the amount of CPU and memory to allocate to the Konnectivity server.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// AddonsSpec defines the enabled addons and their features.
type AddonsSpec struct {
	CoreDNS      *AddonSpec        `json:"coreDNS,omitempty"`
	Konnectivity *KonnectivitySpec `json:"konnectivity,omitempty"`
	KubeProxy    *KubeProxySpec    `json:"kubeProxy,omitempty"`
}

// TenantControlPlaneSpec defines the desired state of TenantControlPlane.
type TenantControlPlaneSpec struct {
	// DataStore allows to specify a DataStore that should be used to store the Kubernetes data for the given Tenant Control Plane.
	// This parameter is optional and acts as an override over the default one which is used by the Kamaji Operator.
	// Migration from a different DataStore to another one is not yet supported and the reconciliation will be blocked.
	DataStore    string       `json:"dataStore,omitempty"`
	ControlPlane ControlPlane `json:"controlPlane"`
	// Kubernetes specification for tenant control plane
	Kubernetes KubernetesSpec `json:"kubernetes"`
	// NetworkProfile specifies how the network is
	NetworkProfile NetworkProfileSpec `json:"networkProfile,omitempty"`
	// Addons contain which addons are enabled
	Addons AddonsSpec `json:"addons,omitempty"`
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
