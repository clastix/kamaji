// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Name               string
	KineContainerImage string
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

	r.Name = "deployment"

	return nil
}

func (r *KubernetesDeploymentResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		address, _, err := tenantControlPlane.AssignedControlPlaneAddress()
		if err != nil {
			return errors.Wrap(err, "cannot create TenantControlPlane Deployment")
		}

		d := builder.Deployment{
			Address:            address,
			DataStore:          r.DataStore,
			KineContainerImage: r.KineContainerImage,
		}
		d.SetLabels(r.resource, utilities.MergeMaps(utilities.CommonLabels(tenantControlPlane.GetName()), tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalMetadata.Labels))
		d.SetAnnotations(r.resource, utilities.MergeMaps(r.resource.Annotations, tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalMetadata.Annotations))
		d.SetTemplateLabels(&r.resource.Spec.Template, r.deploymentTemplateLabels(ctx, tenantControlPlane))
		d.SetStrategy(&r.resource.Spec)
		d.SetSelector(&r.resource.Spec, tenantControlPlane)
		d.SetReplicas(&r.resource.Spec, tenantControlPlane)
		d.ResetKubeAPIServerFlags(r.resource, tenantControlPlane)
		d.SetContainers(&r.resource.Spec.Template.Spec, tenantControlPlane, address)
		d.SetVolumes(&r.resource.Spec.Template.Spec, tenantControlPlane)

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesDeploymentResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesDeploymentResource) GetName() string {
	return r.Name
}

func (r *KubernetesDeploymentResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	switch {
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
		Name:             r.resource.GetName(),
		Namespace:        r.resource.GetNamespace(),
		LastUpdate:       metav1.Now(),
	}

	return nil
}

func (r *KubernetesDeploymentResource) deploymentTemplateLabels(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (labels map[string]string) {
	hash := func(ctx context.Context, namespace, secretName string) (hash string) {
		hash, _ = utilities.SecretHashValue(ctx, r.Client, namespace, secretName)

		return
	}

	labels = map[string]string{
		"kamaji.clastix.io/soot":                                            tenantControlPlane.GetName(),
		"component.kamaji.clastix.io/api-server-certificate":                hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServer.SecretName),
		"component.kamaji.clastix.io/api-server-kubelet-client-certificate": hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName),
		"component.kamaji.clastix.io/ca":                                    hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.CA.SecretName),
		"component.kamaji.clastix.io/controller-manager-kubeconfig":         hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.ControllerManager.SecretName),
		"component.kamaji.clastix.io/front-proxy-ca-certificate":            hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName),
		"component.kamaji.clastix.io/front-proxy-client-certificate":        hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName),
		"component.kamaji.clastix.io/service-account":                       hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.SA.SecretName),
		"component.kamaji.clastix.io/scheduler-kubeconfig":                  hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.Scheduler.SecretName),
	}

	return labels
}

func (r *KubernetesDeploymentResource) isProgressingUpgrade() bool {
	if r.resource.ObjectMeta.GetGeneration() != r.resource.Status.ObservedGeneration {
		return true
	}

	if r.resource.Status.UnavailableReplicas > 0 {
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
