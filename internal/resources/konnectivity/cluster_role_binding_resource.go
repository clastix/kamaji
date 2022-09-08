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
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type ClusterRoleBindingResource struct {
	resource     *rbacv1.ClusterRoleBinding
	Client       client.Client
	Name         string
	tenantClient client.Client
}

func (r *ClusterRoleBindingResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding.Checksum != r.resource.ObjectMeta.GetAnnotations()["checksum"]
}

func (r *ClusterRoleBindingResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *ClusterRoleBindingResource) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot delete the requeste resource")

			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *ClusterRoleBindingResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (err error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CertCommonName,
		},
	}

	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tenantControlPlane); err != nil {
		logger.Error(err, "cannot get Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *ClusterRoleBindingResource) CreateOrUpdate(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate())
}

func (r *ClusterRoleBindingResource) GetName() string {
	return r.Name
}

func (r *ClusterRoleBindingResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true
		tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding = kamajiv1alpha1.ExternalKubernetesObjectStatus{
			Name:     r.resource.GetName(),
			Checksum: r.resource.GetAnnotations()["checksum"],
		}

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding = kamajiv1alpha1.ExternalKubernetesObjectStatus{}
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false

	return nil
}

func (r *ClusterRoleBindingResource) mutate() controllerutil.MutateFn {
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

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		yaml, _ := utilities.EncondeToYaml(r.resource)
		annotations["checksum"] = utilities.MD5Checksum(yaml)

		return nil
	}
}
