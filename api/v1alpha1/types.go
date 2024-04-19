// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import corev1 "k8s.io/api/core/v1"

// +kubebuilder:validation:Enum=AlwaysAdmit;AlwaysDeny;AlwaysPullImages;CertificateApproval;CertificateSigning;CertificateSubjectRestriction;DefaultIngressClass;DefaultStorageClass;DefaultTolerationSeconds;DenyEscalatingExec;DenyExecOnPrivileged;DenyServiceExternalIPs;EventRateLimit;ExtendedResourceToleration;ImagePolicyWebhook;LimitPodHardAntiAffinityTopology;LimitRanger;MutatingAdmissionWebhook;NamespaceAutoProvision;NamespaceExists;NamespaceLifecycle;NodeRestriction;OwnerReferencesPermissionEnforcement;PersistentVolumeClaimResize;PersistentVolumeLabel;PodNodeSelector;PodSecurity;PodSecurityPolicy;PodTolerationRestriction;Priority;ResourceQuota;RuntimeClass;SecurityContextDeny;ServiceAccount;StorageObjectInUseProtection;TaintNodesByCondition;ValidatingAdmissionWebhook
type AdmissionController string

type AdmissionControllers []AdmissionController

func (a AdmissionControllers) ToSlice() []string {
	out := make([]string, len(a))

	for i, v := range a {
		out[i] = string(v)
	}

	return out
}

// +kubebuilder:validation:Enum=systemd;cgroupfs
type CGroupDriver string

func (c CGroupDriver) String() string {
	return (string)(c)
}

const (
	ServiceTypeLoadBalancer       = (ServiceType)(corev1.ServiceTypeLoadBalancer)
	ServiceTypeClusterIP          = (ServiceType)(corev1.ServiceTypeClusterIP)
	ServiceTypeNodePort           = (ServiceType)(corev1.ServiceTypeNodePort)
	KubeconfigSecretKeyAnnotation = "kamaji.clastix.io/kubeconfig-secret-key"
)

// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
type ServiceType corev1.ServiceType
