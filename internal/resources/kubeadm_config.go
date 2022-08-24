// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeadmConfigResource struct {
	resource     *corev1.ConfigMap
	Client       client.Client
	Name         string
	ETCDs        []string
	TmpDirectory string
}

func (r *KubeadmConfigResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.KubeadmConfig.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *KubeadmConfigResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeadmConfigResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
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
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tenantControlPlane))
}

func (r *KubeadmConfigResource) GetName() string {
	return r.Name
}

func (r *KubeadmConfigResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.KubeadmConfig.LastUpdate = metav1.Now()
	tenantControlPlane.Status.KubeadmConfig.Checksum = r.resource.GetAnnotations()["checksum"]
	tenantControlPlane.Status.KubeadmConfig.ConfigmapName = r.resource.GetName()

	return nil
}

func (r *KubeadmConfigResource) getControlPlaneEndpoint(ingress *kamajiv1alpha1.IngressSpec, address string, port int32) string {
	if ingress != nil && len(ingress.Hostname) > 0 {
		return ingress.Hostname
	}

	return fmt.Sprintf("%s:%d", address, port)
}

func (r *KubeadmConfigResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		address, port, err := tenantControlPlane.AssignedControlPlaneAddress()
		if err != nil {
			return err
		}

		r.resource.SetLabels(utilities.KamajiLabels())

		params := kubeadm.Parameters{
			TenantControlPlaneAddress:     address,
			TenantControlPlanePort:        port,
			TenantControlPlaneName:        tenantControlPlane.GetName(),
			TenantControlPlaneNamespace:   tenantControlPlane.GetNamespace(),
			TenantControlPlaneEndpoint:    r.getControlPlaneEndpoint(tenantControlPlane.Spec.ControlPlane.Ingress, address, port),
			TenantControlPlaneCertSANs:    tenantControlPlane.Spec.NetworkProfile.CertSANs,
			TenantControlPlanePodCIDR:     tenantControlPlane.Spec.NetworkProfile.PodCIDR,
			TenantControlPlaneServiceCIDR: tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
			TenantControlPlaneVersion:     tenantControlPlane.Spec.Kubernetes.Version,
			ETCDs:                         r.ETCDs,
			CertificatesDir:               r.TmpDirectory,
		}

		config := kubeadm.CreateKubeadmInitConfiguration(params)
		data, err := kubeadm.GetKubeadmInitConfigurationMap(config)
		if err != nil {
			return err
		}

		r.resource.Data = data

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations["checksum"] = utilities.CalculateConfigMapChecksum(data)
		r.resource.SetAnnotations(annotations)

		if err := ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme()); err != nil {
			return err
		}

		return nil
	}
}
