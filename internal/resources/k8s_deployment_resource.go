// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	builder "github.com/clastix/kamaji/internal/builders/controlplane"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesDeploymentResource struct {
	resource           *appsv1.Deployment
	Client             client.Client
	DataStore          kamajiv1alpha1.DataStore
	KineContainerImage string
}

func (r *KubernetesDeploymentResource) GetHistogram() prometheus.Histogram {
	deploymentCollector = LazyLoadHistogramFromResource(deploymentCollector, r)

	return deploymentCollector
}

func (r *KubernetesDeploymentResource) isStatusEqual(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return r.resource.Status.String() == tenantControlPlane.Status.Kubernetes.Deployment.DeploymentStatus.String()
}

func (r *KubernetesDeploymentResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane) || tenantControlPlane.Spec.Kubernetes.Version != tenantControlPlane.Status.Kubernetes.Version.Version
}

func (r *KubernetesDeploymentResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubernetesDeploymentResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
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

func (r *KubernetesDeploymentResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		(builder.Deployment{
			Client:             r.Client,
			DataStore:          r.DataStore,
			KineContainerImage: r.KineContainerImage,
		}).Build(ctx, r.resource, *tenantControlPlane)

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesDeploymentResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesDeploymentResource) GetName() string {
	return "deployment"
}

func (r *KubernetesDeploymentResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	switch {
	case ptr.Deref(tenantControlPlane.Spec.ControlPlane.Deployment.Replicas, 2) == 0:
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionSleeping
	case !r.isProgressingUpgrade():
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionReady
		tenantControlPlane.Status.Kubernetes.Version.Version = tenantControlPlane.Spec.Kubernetes.Version
	case r.isUpgrading(tenantControlPlane):
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionUpgrading
	case r.isProvisioning(tenantControlPlane):
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionProvisioning
	case r.isNotReady():
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionNotReady
	}

	tenantControlPlane.Status.Kubernetes.Deployment = kamajiv1alpha1.KubernetesDeploymentStatus{
		DeploymentStatus: r.resource.Status,
		Selector:         metav1.FormatLabelSelector(r.resource.Spec.Selector),
		Name:             r.resource.GetName(),
		Namespace:        r.resource.GetNamespace(),
		LastUpdate:       metav1.Now(),
	}

	return nil
}

func (r *KubernetesDeploymentResource) isProgressingUpgrade() bool {
	if r.resource.ObjectMeta.GetGeneration() != r.resource.Status.ObservedGeneration {
		return true
	}

	if r.resource.Status.UnavailableReplicas > 0 {
		return true
	}

	// An update is complete when new pods are ready and old pods deleted.
	desired := ptr.Deref(r.resource.Spec.Replicas, 2)
	if r.resource.Status.UpdatedReplicas != desired {
		return true
	}

	if r.resource.Status.ReadyReplicas != desired {
		return true
	}

	if r.resource.Status.Replicas != desired {
		return true
	}

	if ptr.Deref(r.resource.Status.TerminatingReplicas, 0) > 0 {
		// NOTE: This is currently an alpha field, so on clusters where the DeploymentPodReplacementPolicy
		// feature gate isn't enabled this condition will always be false.
		// Due to its alpha state the semantics may change over time, but the goal here should always be
		// to wait until all old Pods were deleted.
		// See: https://github.com/kubernetes/enhancements/issues/3973
		return true
	}

	return false
}

func (r *KubernetesDeploymentResource) isUpgrading(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Kubernetes.Version.Version) > 0 &&
		tenantControlPlane.Spec.Kubernetes.Version != tenantControlPlane.Status.Kubernetes.Version.Version &&
		r.isProgressingUpgrade()
}

func (r *KubernetesDeploymentResource) isProvisioning(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Kubernetes.Version.Version) == 0
}

func (r *KubernetesDeploymentResource) isNotReady() bool {
	return r.resource.Status.ReadyReplicas == 0
}
