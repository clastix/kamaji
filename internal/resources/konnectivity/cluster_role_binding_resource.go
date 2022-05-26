// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type ClusterRoleBindingResource struct {
	resource     *rbacv1.ClusterRoleBinding
	Client       client.Client
	Name         string
	tenantClient client.Client
}

func (r *ClusterRoleBindingResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding.RV != r.resource.ObjectMeta.ResourceVersion
}

func (r *ClusterRoleBindingResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *ClusterRoleBindingResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *ClusterRoleBindingResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CertCommonName,
		},
	}

	client, err := NewClient(ctx, r, tenantControlPlane)
	if err != nil {
		return err
	}

	r.tenantClient = client

	return nil
}

func (r *ClusterRoleBindingResource) GetClient() client.Client {
	return r.Client
}

func (r *ClusterRoleBindingResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ClusterRoleBindingResource) GetName() string {
	return r.Name
}

func (r *ClusterRoleBindingResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true
		tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding = kamajiv1alpha1.ExternalKubernetesObjectStatus{
			Name: r.resource.GetName(),
			RV:   r.resource.ObjectMeta.ResourceVersion,
		}

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding = kamajiv1alpha1.ExternalKubernetesObjectStatus{}
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false

	return nil
}

func (r *ClusterRoleBindingResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"kubernetes.io/cluster-service":   "true",
				"addonmanager.kubernetes.io/mode": "Reconcile",
			},
		))

		r.resource.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleAuthDelegator,
		}

		r.resource.Subjects = []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     CertCommonName,
			},
		}

		return nil
	}
}
