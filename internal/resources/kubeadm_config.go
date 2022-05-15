// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeadmConfigResource struct {
	resource               *corev1.ConfigMap
	Client                 client.Client
	Scheme                 *runtime.Scheme
	Log                    logr.Logger
	Name                   string
	Port                   int32
	Domain                 string
	PodCIDR                string
	ServiceCIDR            string
	KubernetesVersion      string
	ETCDs                  []string
	ETCDCompactionInterval string
	TmpDirectory           string
}

func (r *KubeadmConfigResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	address, err := tenantControlPlane.GetAddress(ctx, r.Client)
	if err != nil {
		return true
	}

	return !(tenantControlPlane.Status.KubeadmConfig.ResourceVersion == r.resource.ObjectMeta.ResourceVersion &&
		tenantControlPlane.Status.KubeadmConfig.ConfigmapName == r.resource.GetName() &&
		tenantControlPlane.Status.ControlPlaneEndpoint == r.getControlPlaneEndpoint(tenantControlPlane, address))
}

func (r *KubeadmConfigResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeadmConfigResource) CleanUp(ctx context.Context) (bool, error) {
	return false, nil
}

func (r *KubeadmConfigResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubeadmConfigResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *KubeadmConfigResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	address, err := tenantControlPlane.GetAddress(ctx, r.Client)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(tenantControlPlane, address))
}

func (r *KubeadmConfigResource) GetName() string {
	return r.Name
}

func (r *KubeadmConfigResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	address, _ := tenantControlPlane.GetAddress(ctx, r.Client)

	tenantControlPlane.Status.KubeadmConfig.LastUpdate = metav1.Now()
	tenantControlPlane.Status.KubeadmConfig.ResourceVersion = r.resource.ObjectMeta.ResourceVersion
	tenantControlPlane.Status.KubeadmConfig.ConfigmapName = r.resource.GetName()
	tenantControlPlane.Status.ControlPlaneEndpoint = r.getControlPlaneEndpoint(tenantControlPlane, address)

	return nil
}

func (r *KubeadmConfigResource) getControlPlaneEndpoint(tenantControlPlane *kamajiv1alpha1.TenantControlPlane, address string) string {
	if !tenantControlPlane.Spec.ControlPlane.Ingress.Enabled {
		return fmt.Sprintf("%s:%d", address, tenantControlPlane.Spec.NetworkProfile.Port)
	}

	if tenantControlPlane.Spec.ControlPlane.Ingress.Hostname != "" {
		return tenantControlPlane.Spec.ControlPlane.Ingress.Hostname
	}

	return getTenantControllerExternalFQDN(*tenantControlPlane)
}

func (r *KubeadmConfigResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane, address string) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.KamajiLabels())

		params := kubeadm.Parameters{
			TenantControlPlaneName:        tenantControlPlane.GetName(),
			TenantControlPlaneNamespace:   tenantControlPlane.GetNamespace(),
			TenantControlPlaneEndpoint:    r.getControlPlaneEndpoint(tenantControlPlane, address),
			TenantControlPlaneAddress:     address,
			TenantControlPlanePort:        r.Port,
			TenantControlPlaneDomain:      r.Domain,
			TenantControlPlanePodCIDR:     r.PodCIDR,
			TenantControlPlaneServiceCIDR: r.ServiceCIDR,
			TenantControlPlaneVersion:     r.KubernetesVersion,
			ETCDs:                         r.ETCDs,
			ETCDCompactionInterval:        r.ETCDCompactionInterval,
			CertificatesDir:               r.TmpDirectory,
		}

		config := kubeadm.CreateKubeadmInitConfiguration(params)
		data, err := kubeadm.GetKubeadmInitConfigurationMap(config)
		if err != nil {
			return err
		}

		r.resource.Data = data

		if err := ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme()); err != nil {
			return err
		}

		return nil
	}
}
