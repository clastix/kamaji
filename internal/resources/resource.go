// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
)

const (
	OperationResultEnqueueBack controllerutil.OperationResult = "enqueueBack"
)

type ResourceMetric interface {
	GetHistogram() prometheus.Histogram
}

type Resource interface {
	ResourceMetric

	Define(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error
	ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool
	CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error)
	CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error)
	GetName() string
	ShouldStatusBeUpdated(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool
	UpdateTenantControlPlaneStatus(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error
}

type DeletableResource interface {
	GetName() string
	Define(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error
	Delete(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error
}

type KubeadmResource interface {
	GetClient() client.Client
	GetTmpDirectory() string
}

type KubeadmPhaseResource interface {
	Resource
	KubeadmResource
	GetClient() client.Client
	GetKubeadmFunction(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (func(clientset.Interface, *kubeadm.Configuration) ([]byte, error), error)
	GetStatus(tcp *kamajiv1alpha1.TenantControlPlane) (kamajiv1alpha1.KubeadmConfigChecksumDependant, error)
	SetKubeadmConfigChecksum(checksum string)
	GetWatchedObject() client.Object
	GetPredicateFunc() func(obj client.Object) bool
}

type HandlerConfig struct {
	Resource           Resource
	TenantControlPlane *kamajiv1alpha1.TenantControlPlane
}

// Handle handles the given resource and returns a boolean to say if the tenantControlPlane has been modified.
func Handle(ctx context.Context, resource Resource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	startTime := time.Now()
	defer func() {
		resource.GetHistogram().Observe(time.Since(startTime).Seconds())
	}()

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

func getStoredKubeadmConfiguration(ctx context.Context, client client.Client, tmpDirectory string, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeadm.Configuration, error) {
	var configmap corev1.ConfigMap
	namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.KubeadmConfig.ConfigmapName}
	if err := client.Get(ctx, namespacedName, &configmap); err != nil {
		return nil, err
	}

	config, err := kubeadm.GetKubeadmInitConfigurationFromMap(configmap.Data)
	if err != nil {
		return nil, err
	}

	if len(tmpDirectory) > 0 {
		config.InitConfiguration.ClusterConfiguration.CertificatesDir = tmpDirectory
	}

	return config, nil
}

func StripLoadBalancerPortsFromServiceStatus(s corev1.ServiceStatus) corev1.ServiceStatus {
	sanitized := s

	if len(s.LoadBalancer.Ingress) > 0 {
		sanitized.LoadBalancer.Ingress = make([]corev1.LoadBalancerIngress, len(s.LoadBalancer.Ingress))
		copy(sanitized.LoadBalancer.Ingress, s.LoadBalancer.Ingress)
	}

	for i := range sanitized.LoadBalancer.Ingress {
		sanitized.LoadBalancer.Ingress[i].Ports = nil
	}

	return sanitized
}
