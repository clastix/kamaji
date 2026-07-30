// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"

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

var clusterrolebindingCollector prometheus.Histogram

type RBACBootstrap struct {
	Client       client.Client
	resource     *rbacv1.ClusterRoleBinding
	tenantClient client.Client
}

func (r *RBACBootstrap) GetHistogram() prometheus.Histogram {
	clusterrolebindingCollector = resources.LazyLoadHistogramFromResource(clusterrolebindingCollector, r)

	return clusterrolebindingCollector
}

func (r *RBACBootstrap) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	if tcp.Spec.Bootstrap == nil || tcp.Spec.Bootstrap.RBAC == nil || !tcp.Spec.Bootstrap.RBAC.Enabled {
		return false
	}

	// Status is updated when the ClusterRoleBinding name matches
	return tcp.Status.Bootstrap == nil || tcp.Status.Bootstrap.RBAC == nil ||
		tcp.Status.Bootstrap.RBAC.ClusterRoleBinding.Name != r.resource.GetName()
}

func (r *RBACBootstrap) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	// Cleanup if RBAC bootstrap is disabled or not configured
	return tenantControlPlane.Spec.Bootstrap == nil ||
		tenantControlPlane.Spec.Bootstrap.RBAC == nil ||
		!tenantControlPlane.Spec.Bootstrap.RBAC.Enabled
}

func (r *RBACBootstrap) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
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

func (r *RBACBootstrap) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (err error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	// Initialize with a placeholder name; actual name is set in mutate
	r.resource = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kamaji-bootstrap-rbac",
		},
	}

	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tenantControlPlane); err != nil {
		logger.Error(err, "cannot get Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *RBACBootstrap) CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	// Skip if RBAC bootstrap is not enabled
	if tcp.Spec.Bootstrap == nil || tcp.Spec.Bootstrap.RBAC == nil || !tcp.Spec.Bootstrap.RBAC.Enabled {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(tcp))
}

func (r *RBACBootstrap) GetName() string {
	return "rbac-bootstrap-clusterrolebinding"
}

func (r *RBACBootstrap) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Bootstrap == nil {
		tenantControlPlane.Status.Bootstrap = &kamajiv1alpha1.BootstrapStatus{}
	}

	if tenantControlPlane.Spec.Bootstrap == nil || tenantControlPlane.Spec.Bootstrap.RBAC == nil || !tenantControlPlane.Spec.Bootstrap.RBAC.Enabled {
		tenantControlPlane.Status.Bootstrap.RBAC = nil

		return nil
	}

	if tenantControlPlane.Status.Bootstrap.RBAC == nil {
		tenantControlPlane.Status.Bootstrap.RBAC = &kamajiv1alpha1.RBACBootstrapStatus{}
	}

	tenantControlPlane.Status.Bootstrap.RBAC.ClusterRoleBinding.Name = r.resource.GetName()

	return nil
}

func (r *RBACBootstrap) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
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
			Name:     "cluster-admin",
		}

		// Build subjects from AdminUsers and AdminGroups
		var subjects []rbacv1.Subject

		rbacSpec := tenantControlPlane.Spec.Bootstrap.RBAC

		// Add users
		for _, user := range rbacSpec.AdminUsers {
			subjects = append(subjects, rbacv1.Subject{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     user,
			})
		}

		// Add groups
		for _, group := range rbacSpec.AdminGroups {
			subjects = append(subjects, rbacv1.Subject{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.GroupKind,
				Name:     group,
			})
		}

		r.resource.Subjects = subjects

		// Set the resource name based on the first user or group for identification
		switch {
		case len(rbacSpec.AdminUsers) > 0:
			r.resource.Name = fmt.Sprintf("kamaji-%s-admin-user", tenantControlPlane.GetName())
		case len(rbacSpec.AdminGroups) > 0:
			r.resource.Name = fmt.Sprintf("kamaji-%s-admin-group", tenantControlPlane.GetName())
		default:
			r.resource.Name = fmt.Sprintf("kamaji-%s-admin", tenantControlPlane.GetName())
		}

		return nil
	}
}
