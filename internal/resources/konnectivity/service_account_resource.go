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
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type ServiceAccountResource struct {
	Client client.Client

	resource     *corev1.ServiceAccount
	tenantClient client.Client
}

func (r *ServiceAccountResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount.Name) == 0 && len(tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount.Namespace) == 0
}

func (r *ServiceAccountResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil && len(tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount.Name) > 0
}

func (r *ServiceAccountResource) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot delete the requested resource")

		return false, err
	}

	return true, nil
}

func (r *ServiceAccountResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (err error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AgentName,
			Namespace: AgentNamespace,
		},
	}

	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tenantControlPlane); err != nil {
		logger.Error(err, "cannot generate tenant client")

		return err
	}

	return nil
}

func (r *ServiceAccountResource) CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.Konnectivity != nil {
		return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate())
	}

	return controllerutil.OperationResultNone, nil
}

func (r *ServiceAccountResource) GetName() string {
	return "konnectivity-sa"
}

func (r *ServiceAccountResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount = kamajiv1alpha1.ExternalKubernetesObjectStatus{
			Name:      r.resource.GetName(),
			Namespace: r.resource.GetNamespace(),
		}

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.ServiceAccount = kamajiv1alpha1.ExternalKubernetesObjectStatus{}

	return nil
}

func (r *ServiceAccountResource) mutate() controllerutil.MutateFn {
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
