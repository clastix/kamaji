// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type ClusterRoleBindingResource struct {
	Client client.Client

	resource     *rbacv1.ClusterRoleBinding
	tenantClient client.Client
}

func (r *ClusterRoleBindingResource) GetHistogram() prometheus.Histogram {
	clusterrolebindingCollector = resources.LazyLoadHistogramFromResource(clusterrolebindingCollector, r)

	return clusterrolebindingCollector
}

func (r *ClusterRoleBindingResource) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.Konnectivity == nil && tcp.Status.Addons.Konnectivity.ClusterRoleBinding.Name != "" ||
		tcp.Spec.Addons.Konnectivity != nil && (tcp.Status.Addons.Konnectivity.ClusterRoleBinding.Name == "" ||
			tcp.Status.Addons.Konnectivity.ClusterRoleBinding.Name != r.resource.GetName())
}

func (r *ClusterRoleBindingResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil && tenantControlPlane.Status.Addons.Konnectivity.Enabled
}

func (r *ClusterRoleBindingResource) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.tenantClient.Get(ctx, client.ObjectKeyFromObject(r.resource), r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot retrieve the requested resource for deletion")

		return false, err
	}

	if labels := r.resource.GetLabels(); labels == nil || labels[constants.ProjectNameLabelKey] != constants.ProjectNameLabelValue {
		return false, nil
	}

	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot delete the requested resource")

		return false, err
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

func (r *ClusterRoleBindingResource) CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.Konnectivity == nil {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(tcp))
}

func (r *ClusterRoleBindingResource) GetName() string {
	return "konnectivity-clusterrolebinding"
}

func (r *ClusterRoleBindingResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding = kamajiv1alpha1.ExternalKubernetesObjectStatus{}

	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding = kamajiv1alpha1.ExternalKubernetesObjectStatus{
			Name: r.resource.GetName(),
		}
	}

	return nil
}

func (r *ClusterRoleBindingResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()),
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
