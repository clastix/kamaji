//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerCertificatesStatus) DeepCopyInto(out *APIServerCertificatesStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerCertificatesStatus.
func (in *APIServerCertificatesStatus) DeepCopy() *APIServerCertificatesStatus {
	if in == nil {
		return nil
	}
	out := new(APIServerCertificatesStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdditionalMetadata) DeepCopyInto(out *AdditionalMetadata) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdditionalMetadata.
func (in *AdditionalMetadata) DeepCopy() *AdditionalMetadata {
	if in == nil {
		return nil
	}
	out := new(AdditionalMetadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AddonSpec) DeepCopyInto(out *AddonSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AddonSpec.
func (in *AddonSpec) DeepCopy() *AddonSpec {
	if in == nil {
		return nil
	}
	out := new(AddonSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AddonStatus) DeepCopyInto(out *AddonStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AddonStatus.
func (in *AddonStatus) DeepCopy() *AddonStatus {
	if in == nil {
		return nil
	}
	out := new(AddonStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AddonsSpec) DeepCopyInto(out *AddonsSpec) {
	*out = *in
	if in.CoreDNS != nil {
		in, out := &in.CoreDNS, &out.CoreDNS
		*out = new(AddonSpec)
		**out = **in
	}
	if in.Konnectivity != nil {
		in, out := &in.Konnectivity, &out.Konnectivity
		*out = new(KonnectivitySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.KubeProxy != nil {
		in, out := &in.KubeProxy, &out.KubeProxy
		*out = new(AddonSpec)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AddonsSpec.
func (in *AddonsSpec) DeepCopy() *AddonsSpec {
	if in == nil {
		return nil
	}
	out := new(AddonsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AddonsStatus) DeepCopyInto(out *AddonsStatus) {
	*out = *in
	in.CoreDNS.DeepCopyInto(&out.CoreDNS)
	in.KubeProxy.DeepCopyInto(&out.KubeProxy)
	in.Konnectivity.DeepCopyInto(&out.Konnectivity)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AddonsStatus.
func (in *AddonsStatus) DeepCopy() *AddonsStatus {
	if in == nil {
		return nil
	}
	out := new(AddonsStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in AdmissionControllers) DeepCopyInto(out *AdmissionControllers) {
	{
		in := &in
		*out = make(AdmissionControllers, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdmissionControllers.
func (in AdmissionControllers) DeepCopy() AdmissionControllers {
	if in == nil {
		return nil
	}
	out := new(AdmissionControllers)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertificatePrivateKeyPairStatus) DeepCopyInto(out *CertificatePrivateKeyPairStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertificatePrivateKeyPairStatus.
func (in *CertificatePrivateKeyPairStatus) DeepCopy() *CertificatePrivateKeyPairStatus {
	if in == nil {
		return nil
	}
	out := new(CertificatePrivateKeyPairStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertificatesStatus) DeepCopyInto(out *CertificatesStatus) {
	*out = *in
	in.CA.DeepCopyInto(&out.CA)
	in.APIServer.DeepCopyInto(&out.APIServer)
	in.APIServerKubeletClient.DeepCopyInto(&out.APIServerKubeletClient)
	in.FrontProxyCA.DeepCopyInto(&out.FrontProxyCA)
	in.FrontProxyClient.DeepCopyInto(&out.FrontProxyClient)
	in.SA.DeepCopyInto(&out.SA)
	if in.ETCD != nil {
		in, out := &in.ETCD, &out.ETCD
		*out = new(ETCDCertificatesStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertificatesStatus.
func (in *CertificatesStatus) DeepCopy() *CertificatesStatus {
	if in == nil {
		return nil
	}
	out := new(CertificatesStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlane) DeepCopyInto(out *ControlPlane) {
	*out = *in
	in.Deployment.DeepCopyInto(&out.Deployment)
	in.Service.DeepCopyInto(&out.Service)
	in.Ingress.DeepCopyInto(&out.Ingress)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlane.
func (in *ControlPlane) DeepCopy() *ControlPlane {
	if in == nil {
		return nil
	}
	out := new(ControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneComponentsResources) DeepCopyInto(out *ControlPlaneComponentsResources) {
	*out = *in
	if in.APIServer != nil {
		in, out := &in.APIServer, &out.APIServer
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.ControllerManager != nil {
		in, out := &in.ControllerManager, &out.ControllerManager
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.Scheduler != nil {
		in, out := &in.Scheduler, &out.Scheduler
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneComponentsResources.
func (in *ControlPlaneComponentsResources) DeepCopy() *ControlPlaneComponentsResources {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneComponentsResources)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneExtraArgs) DeepCopyInto(out *ControlPlaneExtraArgs) {
	*out = *in
	if in.APIServer != nil {
		in, out := &in.APIServer, &out.APIServer
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ControllerManager != nil {
		in, out := &in.ControllerManager, &out.ControllerManager
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Scheduler != nil {
		in, out := &in.Scheduler, &out.Scheduler
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Kine != nil {
		in, out := &in.Kine, &out.Kine
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneExtraArgs.
func (in *ControlPlaneExtraArgs) DeepCopy() *ControlPlaneExtraArgs {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneExtraArgs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentSpec) DeepCopyInto(out *DeploymentSpec) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(ControlPlaneComponentsResources)
		(*in).DeepCopyInto(*out)
	}
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = new(ControlPlaneExtraArgs)
		(*in).DeepCopyInto(*out)
	}
	in.AdditionalMetadata.DeepCopyInto(&out.AdditionalMetadata)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentSpec.
func (in *DeploymentSpec) DeepCopy() *DeploymentSpec {
	if in == nil {
		return nil
	}
	out := new(DeploymentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCDCertificateStatus) DeepCopyInto(out *ETCDCertificateStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCDCertificateStatus.
func (in *ETCDCertificateStatus) DeepCopy() *ETCDCertificateStatus {
	if in == nil {
		return nil
	}
	out := new(ETCDCertificateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCDCertificatesStatus) DeepCopyInto(out *ETCDCertificatesStatus) {
	*out = *in
	in.APIServer.DeepCopyInto(&out.APIServer)
	in.CA.DeepCopyInto(&out.CA)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCDCertificatesStatus.
func (in *ETCDCertificatesStatus) DeepCopy() *ETCDCertificatesStatus {
	if in == nil {
		return nil
	}
	out := new(ETCDCertificatesStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCDStatus) DeepCopyInto(out *ETCDStatus) {
	*out = *in
	in.Role.DeepCopyInto(&out.Role)
	in.User.DeepCopyInto(&out.User)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCDStatus.
func (in *ETCDStatus) DeepCopy() *ETCDStatus {
	if in == nil {
		return nil
	}
	out := new(ETCDStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalKubernetesObjectStatus) DeepCopyInto(out *ExternalKubernetesObjectStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalKubernetesObjectStatus.
func (in *ExternalKubernetesObjectStatus) DeepCopy() *ExternalKubernetesObjectStatus {
	if in == nil {
		return nil
	}
	out := new(ExternalKubernetesObjectStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressSpec) DeepCopyInto(out *IngressSpec) {
	*out = *in
	in.AdditionalMetadata.DeepCopyInto(&out.AdditionalMetadata)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressSpec.
func (in *IngressSpec) DeepCopy() *IngressSpec {
	if in == nil {
		return nil
	}
	out := new(IngressSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KineMySQLStatus) DeepCopyInto(out *KineMySQLStatus) {
	*out = *in
	out.Config = in.Config
	in.Setup.DeepCopyInto(&out.Setup)
	in.Certificate.DeepCopyInto(&out.Certificate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KineMySQLStatus.
func (in *KineMySQLStatus) DeepCopy() *KineMySQLStatus {
	if in == nil {
		return nil
	}
	out := new(KineMySQLStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KonnectivitySpec) DeepCopyInto(out *KonnectivitySpec) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KonnectivitySpec.
func (in *KonnectivitySpec) DeepCopy() *KonnectivitySpec {
	if in == nil {
		return nil
	}
	out := new(KonnectivitySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KonnectivityStatus) DeepCopyInto(out *KonnectivityStatus) {
	*out = *in
	in.Certificate.DeepCopyInto(&out.Certificate)
	in.Kubeconfig.DeepCopyInto(&out.Kubeconfig)
	in.ServiceAccount.DeepCopyInto(&out.ServiceAccount)
	in.ClusterRoleBinding.DeepCopyInto(&out.ClusterRoleBinding)
	in.Agent.DeepCopyInto(&out.Agent)
	in.Service.DeepCopyInto(&out.Service)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KonnectivityStatus.
func (in *KonnectivityStatus) DeepCopy() *KonnectivityStatus {
	if in == nil {
		return nil
	}
	out := new(KonnectivityStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeadmConfigStatus) DeepCopyInto(out *KubeadmConfigStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeadmConfigStatus.
func (in *KubeadmConfigStatus) DeepCopy() *KubeadmConfigStatus {
	if in == nil {
		return nil
	}
	out := new(KubeadmConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeadmPhaseStatus) DeepCopyInto(out *KubeadmPhaseStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeadmPhaseStatus.
func (in *KubeadmPhaseStatus) DeepCopy() *KubeadmPhaseStatus {
	if in == nil {
		return nil
	}
	out := new(KubeadmPhaseStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeadmPhasesStatus) DeepCopyInto(out *KubeadmPhasesStatus) {
	*out = *in
	in.UploadConfigKubeadm.DeepCopyInto(&out.UploadConfigKubeadm)
	in.UploadConfigKubelet.DeepCopyInto(&out.UploadConfigKubelet)
	in.BootstrapToken.DeepCopyInto(&out.BootstrapToken)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeadmPhasesStatus.
func (in *KubeadmPhasesStatus) DeepCopy() *KubeadmPhasesStatus {
	if in == nil {
		return nil
	}
	out := new(KubeadmPhasesStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeconfigStatus) DeepCopyInto(out *KubeconfigStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeconfigStatus.
func (in *KubeconfigStatus) DeepCopy() *KubeconfigStatus {
	if in == nil {
		return nil
	}
	out := new(KubeconfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeconfigsStatus) DeepCopyInto(out *KubeconfigsStatus) {
	*out = *in
	in.Admin.DeepCopyInto(&out.Admin)
	in.ControllerManager.DeepCopyInto(&out.ControllerManager)
	in.Scheduler.DeepCopyInto(&out.Scheduler)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeconfigsStatus.
func (in *KubeconfigsStatus) DeepCopy() *KubeconfigsStatus {
	if in == nil {
		return nil
	}
	out := new(KubeconfigsStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeletSpec) DeepCopyInto(out *KubeletSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeletSpec.
func (in *KubeletSpec) DeepCopy() *KubeletSpec {
	if in == nil {
		return nil
	}
	out := new(KubeletSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesDeploymentStatus) DeepCopyInto(out *KubernetesDeploymentStatus) {
	*out = *in
	in.DeploymentStatus.DeepCopyInto(&out.DeploymentStatus)
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesDeploymentStatus.
func (in *KubernetesDeploymentStatus) DeepCopy() *KubernetesDeploymentStatus {
	if in == nil {
		return nil
	}
	out := new(KubernetesDeploymentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesIngressStatus) DeepCopyInto(out *KubernetesIngressStatus) {
	*out = *in
	in.IngressStatus.DeepCopyInto(&out.IngressStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesIngressStatus.
func (in *KubernetesIngressStatus) DeepCopy() *KubernetesIngressStatus {
	if in == nil {
		return nil
	}
	out := new(KubernetesIngressStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesServiceStatus) DeepCopyInto(out *KubernetesServiceStatus) {
	*out = *in
	in.ServiceStatus.DeepCopyInto(&out.ServiceStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesServiceStatus.
func (in *KubernetesServiceStatus) DeepCopy() *KubernetesServiceStatus {
	if in == nil {
		return nil
	}
	out := new(KubernetesServiceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesSpec) DeepCopyInto(out *KubernetesSpec) {
	*out = *in
	out.Kubelet = in.Kubelet
	if in.AdmissionControllers != nil {
		in, out := &in.AdmissionControllers, &out.AdmissionControllers
		*out = make(AdmissionControllers, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesSpec.
func (in *KubernetesSpec) DeepCopy() *KubernetesSpec {
	if in == nil {
		return nil
	}
	out := new(KubernetesSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesStatus) DeepCopyInto(out *KubernetesStatus) {
	*out = *in
	in.Version.DeepCopyInto(&out.Version)
	in.Deployment.DeepCopyInto(&out.Deployment)
	in.Service.DeepCopyInto(&out.Service)
	in.Ingress.DeepCopyInto(&out.Ingress)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesStatus.
func (in *KubernetesStatus) DeepCopy() *KubernetesStatus {
	if in == nil {
		return nil
	}
	out := new(KubernetesStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesVersion) DeepCopyInto(out *KubernetesVersion) {
	*out = *in
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(KubernetesVersionStatus)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesVersion.
func (in *KubernetesVersion) DeepCopy() *KubernetesVersion {
	if in == nil {
		return nil
	}
	out := new(KubernetesVersion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkProfileSpec) DeepCopyInto(out *NetworkProfileSpec) {
	*out = *in
	if in.CertSANs != nil {
		in, out := &in.CertSANs, &out.CertSANs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DNSServiceIPs != nil {
		in, out := &in.DNSServiceIPs, &out.DNSServiceIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkProfileSpec.
func (in *NetworkProfileSpec) DeepCopy() *NetworkProfileSpec {
	if in == nil {
		return nil
	}
	out := new(NetworkProfileSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PublicKeyPrivateKeyPairStatus) DeepCopyInto(out *PublicKeyPrivateKeyPairStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PublicKeyPrivateKeyPairStatus.
func (in *PublicKeyPrivateKeyPairStatus) DeepCopy() *PublicKeyPrivateKeyPairStatus {
	if in == nil {
		return nil
	}
	out := new(PublicKeyPrivateKeyPairStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SQLCertificateStatus) DeepCopyInto(out *SQLCertificateStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SQLCertificateStatus.
func (in *SQLCertificateStatus) DeepCopy() *SQLCertificateStatus {
	if in == nil {
		return nil
	}
	out := new(SQLCertificateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SQLConfigStatus) DeepCopyInto(out *SQLConfigStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SQLConfigStatus.
func (in *SQLConfigStatus) DeepCopy() *SQLConfigStatus {
	if in == nil {
		return nil
	}
	out := new(SQLConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SQLSetupStatus) DeepCopyInto(out *SQLSetupStatus) {
	*out = *in
	in.LastUpdate.DeepCopyInto(&out.LastUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SQLSetupStatus.
func (in *SQLSetupStatus) DeepCopy() *SQLSetupStatus {
	if in == nil {
		return nil
	}
	out := new(SQLSetupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceSpec) DeepCopyInto(out *ServiceSpec) {
	*out = *in
	in.AdditionalMetadata.DeepCopyInto(&out.AdditionalMetadata)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceSpec.
func (in *ServiceSpec) DeepCopy() *ServiceSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StorageStatus) DeepCopyInto(out *StorageStatus) {
	*out = *in
	if in.ETCD != nil {
		in, out := &in.ETCD, &out.ETCD
		*out = new(ETCDStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.KineMySQL != nil {
		in, out := &in.KineMySQL, &out.KineMySQL
		*out = new(KineMySQLStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StorageStatus.
func (in *StorageStatus) DeepCopy() *StorageStatus {
	if in == nil {
		return nil
	}
	out := new(StorageStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TenantControlPlane) DeepCopyInto(out *TenantControlPlane) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TenantControlPlane.
func (in *TenantControlPlane) DeepCopy() *TenantControlPlane {
	if in == nil {
		return nil
	}
	out := new(TenantControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TenantControlPlane) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TenantControlPlaneList) DeepCopyInto(out *TenantControlPlaneList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TenantControlPlane, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TenantControlPlaneList.
func (in *TenantControlPlaneList) DeepCopy() *TenantControlPlaneList {
	if in == nil {
		return nil
	}
	out := new(TenantControlPlaneList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TenantControlPlaneList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TenantControlPlaneSpec) DeepCopyInto(out *TenantControlPlaneSpec) {
	*out = *in
	in.ControlPlane.DeepCopyInto(&out.ControlPlane)
	in.Kubernetes.DeepCopyInto(&out.Kubernetes)
	in.NetworkProfile.DeepCopyInto(&out.NetworkProfile)
	in.Addons.DeepCopyInto(&out.Addons)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TenantControlPlaneSpec.
func (in *TenantControlPlaneSpec) DeepCopy() *TenantControlPlaneSpec {
	if in == nil {
		return nil
	}
	out := new(TenantControlPlaneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TenantControlPlaneStatus) DeepCopyInto(out *TenantControlPlaneStatus) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
	in.Certificates.DeepCopyInto(&out.Certificates)
	in.KubeConfig.DeepCopyInto(&out.KubeConfig)
	in.Kubernetes.DeepCopyInto(&out.Kubernetes)
	in.KubeadmConfig.DeepCopyInto(&out.KubeadmConfig)
	in.KubeadmPhase.DeepCopyInto(&out.KubeadmPhase)
	in.Addons.DeepCopyInto(&out.Addons)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TenantControlPlaneStatus.
func (in *TenantControlPlaneStatus) DeepCopy() *TenantControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(TenantControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}
