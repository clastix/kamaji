// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0
package konnectivity

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type EgressSelectorConfigurationResource struct {
	resource *corev1.ConfigMap
	Client   client.Client
	Name     string
}

func (r *EgressSelectorConfigurationResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *EgressSelectorConfigurationResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *EgressSelectorConfigurationResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *EgressSelectorConfigurationResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *EgressSelectorConfigurationResource) GetName() string {
	return r.Name
}

func (r *EgressSelectorConfigurationResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.ConfigMap.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *EgressSelectorConfigurationResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true
		tenantControlPlane.Status.Addons.Konnectivity.ConfigMap.Name = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.ConfigMap.Checksum = r.resource.GetAnnotations()["checksum"]

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false
	tenantControlPlane.Status.Addons.Konnectivity.ConfigMap = kamajiv1alpha1.KonnectivityConfigMap{}

	return nil
}

func (r *EgressSelectorConfigurationResource) mutate(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) func() error {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels()))

		configuration := &apiserverv1alpha1.EgressSelectorConfiguration{
			TypeMeta: metav1.TypeMeta{
				Kind:       egressSelectorConfigurationKind,
				APIVersion: apiServerAPIVersion,
			},
			EgressSelections: []apiserverv1alpha1.EgressSelection{
				{
					Name: egressSelectorConfigurationName,
					Connection: apiserverv1alpha1.Connection{
						ProxyProtocol: apiserverv1alpha1.ProtocolGRPC,
						Transport: &apiserverv1alpha1.Transport{
							UDS: &apiserverv1alpha1.UDSTransport{
								UDSName: defaultUDSName,
							},
						},
					},
				},
			},
		}

		yamlConfiguration, err := utilities.EncondeToYaml(configuration)
		if err != nil {
			return err
		}

		r.resource.Data = map[string]string{
			"egress-selector-configuration.yaml": string(yamlConfiguration),
		}

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations["checksum"] = utilities.MD5Checksum(yamlConfiguration)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *EgressSelectorConfigurationResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}
