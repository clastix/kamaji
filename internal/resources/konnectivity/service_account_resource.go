// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type ServiceAccountResource struct {
	resource     *corev1.ServiceAccount
	Client       client.Client
	Name         string
	tenantClient client.Client
}

func (r *ServiceAccountResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount.Namespace != r.resource.GetNamespace() ||
		tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount.RV != r.resource.ObjectMeta.ResourceVersion
}

func (r *ServiceAccountResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *ServiceAccountResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *ServiceAccountResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "konnectivity-agent",
			Namespace: agentNamespace,
		},
	}

	client, err := utilities.GetTenantClient(ctx, r.Client, tenantControlPlane)
	if err != nil {
		return err
	}

	r.tenantClient = client

	return nil
}

func (r *ServiceAccountResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ServiceAccountResource) GetName() string {
	return r.Name
}

func (r *ServiceAccountResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount = kamajiv1alpha1.ExternalKubernetesObjectStatus{
			Name:      r.resource.GetName(),
			Namespace: r.resource.GetNamespace(),
			RV:        r.resource.ObjectMeta.ResourceVersion,
		}
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false
	tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount = kamajiv1alpha1.ExternalKubernetesObjectStatus{}

	return nil
}

func (r *ServiceAccountResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"kubernetes.io/cluster-service":   "true",
				"addonmanager.kubernetes.io/mode": "Reconcile",
			},
		))

		return nil
	}
}
