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
	config, resourceVersion, err := getKubeadmConfiguration(ctx, r, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	kubeconfig, err := utilities.GetKubeconfig(ctx, r.GetClient(), tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	config.Kubeconfig = *kubeconfig
	config.Parameters = kubeadm.Parameters{
		TenantControlPlaneName:         tenantControlPlane.GetName(),
		TenantDNSServiceIPs:            tenantControlPlane.Spec.NetworkProfile.DNSServiceIPs,
		TenantControlPlaneVersion:      tenantControlPlane.Spec.Kubernetes.Version,
		TenantControlPlanePodCIDR:      tenantControlPlane.Spec.NetworkProfile.PodCIDR,
		TenantControlPlaneAddress:      tenantControlPlane.Spec.NetworkProfile.Address,
		TenantControlPlanePort:         tenantControlPlane.Spec.NetworkProfile.Port,
		TenantControlPlaneCGroupDriver: tenantControlPlane.Spec.Kubernetes.Kubelet.CGroupFS.String(),
	}

	status, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	storedResourceVersion := status.GetKubeadmConfigResourceVersion()
	if resourceVersion == storedResourceVersion {
		r.SetKubeadmConfigResourceVersion(resourceVersion)

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

	r.SetKubeadmConfigResourceVersion(resourceVersion)

	if storedResourceVersion == "" {
		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultUpdated, nil
}
