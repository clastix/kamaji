// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	builder "github.com/clastix/kamaji/internal/builders/controlplane"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesDeploymentResource struct {
	resource *appsv1.Deployment

	Builder builder.Konnectivity
	Client  client.Client
}

func (r *KubernetesDeploymentResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	switch {
	case tenantControlPlane.Spec.Addons.Konnectivity == nil && tenantControlPlane.Status.Addons.Konnectivity.Enabled:
		fallthrough
	case tenantControlPlane.Spec.Addons.Konnectivity != nil && !tenantControlPlane.Status.Addons.Konnectivity.Enabled:
		return true
	default:
		return false
	}
}

func (r *KubernetesDeploymentResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil && tenantControlPlane.Status.Addons.Konnectivity.Enabled
}

func (r *KubernetesDeploymentResource) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx)

	logger.Info("performing clean-up from Deployment of Konnectivity")

	res, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, func() error {
		logger.Info("removing Konnectivity container")
		r.Builder.RemovingContainer(&r.resource.Spec.Template.Spec)

		logger.Info("removing egress selector configuration file from kube-apiserver container")
		r.Builder.RemovingKubeAPIServerContainerArg(&r.resource.Spec.Template.Spec)

		logger.Info("removing Konnectivity volumes")
		r.Builder.RemovingVolumes(&r.resource.Spec.Template.Spec)

		logger.Info("removing Konnectivity volume mounts")
		r.Builder.RemovingVolumeMounts(&r.resource.Spec.Template.Spec)

		return nil
	})

	return res == controllerutil.OperationResultUpdated, err
}

func (r *KubernetesDeploymentResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesDeploymentResource) mutate(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() (err error) {
		// If konnectivity is disabled, no operation is required:
		// removal of the container will be performed by clean-up.
		if tenantControlPlane.Spec.Addons.Konnectivity == nil {
			return nil
		}

		if len(r.resource.Spec.Template.Spec.Containers) == 0 {
			return fmt.Errorf("the Deployment resource is not ready to be mangled for Konnectivity server enrichment")
		}

		r.Builder.BuildKonnectivityContainer(tenantControlPlane.Spec.Addons.Konnectivity, tenantControlPlane.Spec.ControlPlane.Deployment.Replicas, &r.resource.Spec.Template.Spec)
		r.Builder.BuildVolumeMounts(&r.resource.Spec.Template.Spec)
		r.Builder.BuildVolumes(tenantControlPlane.Status.Addons.Konnectivity, &r.resource.Spec.Template.Spec)

		r.Client.Scheme().Default(r.resource)

		return nil
	}
}

func (r *KubernetesDeploymentResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesDeploymentResource) GetName() string {
	return "konnectivity-deployment"
}

func (r *KubernetesDeploymentResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = tenantControlPlane.Spec.Addons.Konnectivity != nil

	return nil
}
