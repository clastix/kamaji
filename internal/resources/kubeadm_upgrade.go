// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm/printers"
	kamajiupgrade "github.com/clastix/kamaji/internal/upgrade"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesUpgrade struct {
	Client  client.Client
	upgrade upgrade.Upgrade

	inProgress bool
}

func (k *KubernetesUpgrade) GetHistogram() prometheus.Histogram {
	kubeadmupgradeCollector = LazyLoadHistogramFromResource(kubeadmupgradeCollector, k)

	return kubeadmupgradeCollector
}

func (k *KubernetesUpgrade) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	k.upgrade = upgrade.Upgrade{
		Before: upgrade.ClusterState{
			KubeVersion: tenantControlPlane.Status.Kubernetes.Version.Version,
		},
		After: upgrade.ClusterState{
			KubeVersion: tenantControlPlane.Spec.Kubernetes.Version,
		},
	}

	return nil
}

func (k *KubernetesUpgrade) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (k *KubernetesUpgrade) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (k *KubernetesUpgrade) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	// A new installation, no need to upgrade
	if len(tenantControlPlane.Status.Kubernetes.Version.Version) == 0 {
		k.inProgress = false

		return controllerutil.OperationResultNone, nil
	}
	// No version change, no need to upgrade
	if tenantControlPlane.Status.Kubernetes.Version.Version == tenantControlPlane.Spec.Kubernetes.Version {
		k.inProgress = false

		return controllerutil.OperationResultNone, nil
	}
	// An upgrade is in progress, let it go
	if status := tenantControlPlane.Status.Kubernetes.Version.Status; status != nil && *status == kamajiv1alpha1.VersionUpgrading {
		return controllerutil.OperationResultNone, nil
	}
	// Checking if the upgrade is allowed, or not
	clientSet, err := utilities.GetTenantClientSet(ctx, k.Client, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("cannot create REST client required for Kubernetes upgrade plan: %w", err)
	}

	var coreDNSVersion string
	if tenantControlPlane.Spec.Addons.CoreDNS != nil {
		coreDNSVersion = tenantControlPlane.Spec.Addons.CoreDNS.ImageTag
	}

	versionGetter := kamajiupgrade.NewKamajiKubeVersionGetter(clientSet, tenantControlPlane.Status.Kubernetes.Version.Version, coreDNSVersion, tenantControlPlane.Status.Kubernetes.Version.Status)

	if _, err = upgrade.GetAvailableUpgrades(versionGetter, false, false, &printers.Discard{}); err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("cannot retrieve available Upgrades for Kubernetes upgrade plan: %w", err)
	}

	if err = k.isUpgradable(); err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("the required upgrade plan is not available")
	}

	if ptr.Deref(tenantControlPlane.Spec.ControlPlane.Deployment.Replicas, 0) > 0 {
		k.inProgress = true
	}

	return controllerutil.OperationResultNone, nil
}

func (k *KubernetesUpgrade) GetName() string {
	return "upgrade"
}

func (k *KubernetesUpgrade) ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool {
	return k.inProgress
}

func (k *KubernetesUpgrade) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if k.inProgress {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionUpgrading
	}

	if tenantControlPlane.Spec.Kubernetes.Version == tenantControlPlane.Status.Kubernetes.Version.Version {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionReady
	}

	return nil
}

func (k *KubernetesUpgrade) isUpgradable() error {
	newK8sVersion, err := version.ParseSemantic(k.upgrade.After.KubeVersion)
	if err != nil {
		return fmt.Errorf("unable to parse normalized version %q as a semantic version: %w", k.upgrade.After.KubeVersion, err)
	}

	oldK8sVersion, err := version.ParseSemantic(k.upgrade.Before.KubeVersion)
	if err != nil {
		return fmt.Errorf("unable to parse normalized version %q as a semantic version: %w", k.upgrade.After.KubeVersion, err)
	}

	if newK8sVersion.Minor() < oldK8sVersion.Minor() {
		return fmt.Errorf("cannot downgrade to a previous minor version of Kubernetes")
	}
	// Patch upgrades are allowed
	if newK8sVersion.Minor() == oldK8sVersion.Minor() {
		return nil
	}
	// Following minor release upgrades are allowed
	if newK8sVersion.Minor() == oldK8sVersion.Minor()+1 {
		return nil
	}

	return fmt.Errorf("an upgrade to a non consecutive Kubernetes minor release is forbidden")
}
