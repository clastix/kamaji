// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

func KubeadmPhaseCreate(ctx context.Context, r KubeadmPhaseResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	config, err := getStoredKubeadmConfiguration(ctx, r, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	kubeconfig, err := utilities.GetKubeconfig(ctx, r.GetClient(), tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	address, _, err := tenantControlPlane.AssignedControlPlaneAddress()
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	config.Kubeconfig = *kubeconfig
	config.Parameters = kubeadm.Parameters{
		TenantControlPlaneName:         tenantControlPlane.GetName(),
		TenantDNSServiceIPs:            tenantControlPlane.Spec.NetworkProfile.DNSServiceIPs,
		TenantControlPlaneVersion:      tenantControlPlane.Spec.Kubernetes.Version,
		TenantControlPlanePodCIDR:      tenantControlPlane.Spec.NetworkProfile.PodCIDR,
		TenantControlPlaneAddress:      address,
		TenantControlPlaneCertSANs:     tenantControlPlane.Spec.NetworkProfile.CertSANs,
		TenantControlPlanePort:         tenantControlPlane.Spec.NetworkProfile.Port,
		TenantControlPlaneCGroupDriver: tenantControlPlane.Spec.Kubernetes.Kubelet.CGroupFS.String(),
	}

	// If CoreDNS addon is enabled and with an override, adding these to the kubeadm init configuration
	if coreDNS := tenantControlPlane.Spec.Addons.CoreDNS; coreDNS != nil {
		config.Parameters.CoreDNSOptions = &kubeadm.AddonOptions{}

		if len(coreDNS.ImageRepository) > 0 {
			config.Parameters.CoreDNSOptions.Repository = coreDNS.ImageRepository
		}

		if len(coreDNS.ImageRepository) > 0 {
			config.Parameters.CoreDNSOptions.Tag = coreDNS.ImageTag
		}
	}
	// If the kube-proxy addon is enabled and with overrides, adding it to the kubeadm parameters
	if kubeProxy := tenantControlPlane.Spec.Addons.KubeProxy; kubeProxy != nil {
		config.Parameters.KubeProxyOptions = &kubeadm.AddonOptions{}

		if len(kubeProxy.ImageRepository) > 0 {
			config.Parameters.KubeProxyOptions.Repository = kubeProxy.ImageRepository
		} else {
			config.Parameters.KubeProxyOptions.Repository = "k8s.gcr.io"
		}

		if len(kubeProxy.ImageTag) > 0 {
			config.Parameters.KubeProxyOptions.Tag = kubeProxy.ImageTag
		} else {
			config.Parameters.KubeProxyOptions.Tag = tenantControlPlane.Spec.Kubernetes.Version
		}
	}

	checksum := config.Checksum()

	status, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if checksum == status.GetChecksum() {
		r.SetKubeadmConfigChecksum(checksum)

		return controllerutil.OperationResultNone, nil
	}

	client, err := utilities.GetTenantRESTClient(ctx, r.GetClient(), tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	fun, err := r.GetKubeadmFunction()
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	if err = fun(client, config); err != nil {
		return controllerutil.OperationResultNone, err
	}

	r.SetKubeadmConfigChecksum(checksum)

	if checksum == "" {
		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultUpdated, nil
}
