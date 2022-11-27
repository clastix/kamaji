// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
)

type Resource interface {
	Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error
	ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool
	CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error)
	CreateOrUpdate(context.Context, *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error)
	GetName() string
	ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool
	UpdateTenantControlPlaneStatus(context.Context, *kamajiv1alpha1.TenantControlPlane) error
}

type DeletableResource interface {
	GetName() string
	Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error
	Delete(context.Context, *kamajiv1alpha1.TenantControlPlane) error
}

type KubeadmResource interface {
	Resource
	GetClient() client.Client
	GetTmpDirectory() string
}

type KubeadmPhaseResource interface {
	KubeadmResource
	GetClient() client.Client
	GetKubeadmFunction() (func(clientset.Interface, *kubeadm.Configuration) error, error)
	GetStatus(*kamajiv1alpha1.TenantControlPlane) (kamajiv1alpha1.KubeadmConfigChecksumDependant, error)
	SetKubeadmConfigChecksum(string)
}

type HandlerConfig struct {
	Resource           Resource
	TenantControlPlane *kamajiv1alpha1.TenantControlPlane
}

// Handle handles the given resource and returns a boolean to say if the tenantControlPlane has been modified.
func Handle(ctx context.Context, resource Resource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if err := resource.Define(ctx, tenantControlPlane); err != nil {
		return "", err
	}

	if !resource.ShouldCleanup(tenantControlPlane) {
		return createOrUpdate(ctx, resource, tenantControlPlane)
	}

	cleanUp, err := resource.CleanUp(ctx, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if cleanUp {
		return controllerutil.OperationResultUpdated, nil
	}

	return controllerutil.OperationResultNone, err
}

// HandleDeletion handles the deletion of the given resource.
func HandleDeletion(ctx context.Context, resource DeletableResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if err := resource.Define(ctx, tenantControlPlane); err != nil {
		return err
	}

	return resource.Delete(ctx, tenantControlPlane)
}

func createOrUpdate(ctx context.Context, resource Resource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	result, err := resource.CreateOrUpdate(ctx, tenantControlPlane)
	if err != nil {
		return "", err
	}

	if result == controllerutil.OperationResultNone && resource.ShouldStatusBeUpdated(ctx, tenantControlPlane) {
		return controllerutil.OperationResultUpdatedStatusOnly, nil
	}

	return result, nil
}

func getStoredKubeadmConfiguration(ctx context.Context, r KubeadmResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeadm.Configuration, error) {
	var configmap corev1.ConfigMap
	namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.KubeadmConfig.ConfigmapName}
	if err := r.GetClient().Get(ctx, namespacedName, &configmap); err != nil {
		return nil, err
	}

	config, err := kubeadm.GetKubeadmInitConfigurationFromMap(configmap.Data)
	if err != nil {
		return nil, err
	}

	tmpDirectory := r.GetTmpDirectory()
	if tmpDirectory != "" {
		config.InitConfiguration.ClusterConfiguration.CertificatesDir = tmpDirectory
	}

	return config, nil
}
