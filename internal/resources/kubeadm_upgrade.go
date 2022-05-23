// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
	kamajiupgrade "github.com/clastix/kamaji/internal/upgrade"
)

type KubernetesUpgrade struct {
	Name    string
	Client  client.Client
	upgrade upgrade.Upgrade

	inProgress bool
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

func (k *KubernetesUpgrade) CreateOrUpdate(ctx context.Context, plane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	// A new installation, no need to upgrade
	if len(plane.Status.Kubernetes.Version.Version) == 0 {
		k.inProgress = false

		return controllerutil.OperationResultNone, nil
	}
	// No version change, no need to upgrade
	if plane.Status.Kubernetes.Version.Version == plane.Spec.Kubernetes.Version {
		k.inProgress = false

		return controllerutil.OperationResultNone, nil
	}
	// An upgrade is in progress, let it go
	if status := plane.Status.Kubernetes.Version.Status; status != nil && *status == kamajiv1alpha1.VersionUpgrading {
		return controllerutil.OperationResultNone, nil
	}
	// Checking if the upgrade is allowed, or not
	restClient, err := k.getRESTClient(ctx, plane)
	if err != nil {
		return controllerutil.OperationResultNone, errors.Wrap(err, "cannot create REST client required for Kubernetes upgrade plan")
	}

	versionGetter := kamajiupgrade.NewKamajiKubeVersionGetter(restClient)

	if _, err = upgrade.GetAvailableUpgrades(versionGetter, false, false, true, restClient, ""); err != nil {
		return controllerutil.OperationResultNone, errors.Wrap(err, "cannot retrieve available Upgrades for Kubernetes upgrade plan")
	}

	if err = k.isUpgradable(); err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("the required upgrade plan is not available")
	}

	k.inProgress = true

	return controllerutil.OperationResultNone, nil
}

func (k *KubernetesUpgrade) GetName() string {
	return k.Name
}

func (k *KubernetesUpgrade) ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool {
	return k.inProgress
}

func (k *KubernetesUpgrade) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if k.inProgress {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionUpgrading
	}

	if tenantControlPlane.Spec.Kubernetes.Version == tenantControlPlane.Status.Kubernetes.Version.Version {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionReady
	}

	return nil
}

func (k *KubernetesUpgrade) getKubeconfigSecret(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*corev1.Secret, error) {
	kubeconfigSecretName := tenantControlPlane.Status.KubeConfig.Admin.SecretName
	namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: kubeconfigSecretName}
	secret := &corev1.Secret{}
	if err := k.Client.Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func (k *KubernetesUpgrade) getKubeconfig(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeconfigutil.Kubeconfig, error) {
	secretKubeconfig, err := k.getKubeconfigSecret(ctx, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	bytes, ok := secretKubeconfig.Data[kubeconfigAdminKeyName]
	if !ok {
		return nil, fmt.Errorf("%s is not into kubeconfig secret", kubeconfigAdminKeyName)
	}

	return kubeconfigutil.GetKubeconfigFromBytes(bytes)
}

func (k *KubernetesUpgrade) getRESTClient(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*clientset.Clientset, error) {
	config, err := k.getRESTClientConfig(ctx, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(config)
}

func (k *KubernetesUpgrade) getRESTClientConfig(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*restclient.Config, error) {
	kubeconfig, err := k.getKubeconfig(ctx, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	config := &restclient.Config{
		Host: fmt.Sprintf("https://%s:%d", getTenantControllerInternalFQDN(*tenantControlPlane), tenantControlPlane.Spec.NetworkProfile.Port),
		TLSClientConfig: restclient.TLSClientConfig{
			CAData:   kubeconfig.Clusters[0].Cluster.CertificateAuthorityData,
			CertData: kubeconfig.AuthInfos[0].AuthInfo.ClientCertificateData,
			KeyData:  kubeconfig.AuthInfos[0].AuthInfo.ClientKeyData,
		},
		Timeout: time.Second * kubeadmPhaseTimeout,
	}

	return config, nil
}

func (k *KubernetesUpgrade) isUpgradable() error {
	newK8sVersion, err := version.ParseSemantic(k.upgrade.After.KubeVersion)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to parse normalized version %q as a semantic version", k.upgrade.After.KubeVersion))
	}

	oldK8sVersion, err := version.ParseSemantic(k.upgrade.Before.KubeVersion)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to parse normalized version %q as a semantic version", k.upgrade.After.KubeVersion))
	}

	if newK8sVersion.Minor() < oldK8sVersion.Minor() {
		return fmt.Errorf("cannot downgrade to a previous minor version of Kubernetes")
	}
	// Patch upgrades are allowed
	if newK8sVersion.Minor() == oldK8sVersion.Minor() {
		return nil
	}
	// Following minor release upgrades are allowed
	if newK8sVersion.Minor() > oldK8sVersion.WithMinor(oldK8sVersion.Minor()+1).Minor() {
		return nil
	}

	return fmt.Errorf("an upgrade to a non consecutive Kubernetes minor release is forbidden")
}
