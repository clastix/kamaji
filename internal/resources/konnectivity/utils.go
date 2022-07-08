// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
)

// TODO: refactor and merge with /internal/resources/kubeadm_utils.go
// Logic is pretty close
// https://github.com/clastix/kamaji/issues/63

const (
	kubeconfigAdminKeyName = "admin.conf"
	timeout                = 10 // seconds
	kubeSystemNamespace    = "kube-system"
)

type ExternalKubernetesResource interface {
	GetClient() client.Client
}

func NewClient(ctx context.Context, r ExternalKubernetesResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (client.Client, error) {
	options := client.Options{}
	config, err := getRESTClientConfig(ctx, r, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return client.New(config, options)
}

func getKubeconfigSecret(ctx context.Context, r ExternalKubernetesResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*corev1.Secret, error) {
	kubeconfigSecretName := tenantControlPlane.Status.KubeConfig.Admin.SecretName
	namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: kubeconfigSecretName}
	secret := &corev1.Secret{}
	if err := r.GetClient().Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func getKubeconfig(ctx context.Context, r ExternalKubernetesResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeconfigutil.Kubeconfig, error) {
	secretKubeconfig, err := getKubeconfigSecret(ctx, r, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	bytes, ok := secretKubeconfig.Data[kubeconfigAdminKeyName]
	if !ok {
		return nil, fmt.Errorf("%s is not into kubeconfig secret", kubeconfigAdminKeyName)
	}

	return kubeconfigutil.GetKubeconfigFromBytes(bytes)
}

func getRESTClientConfig(ctx context.Context, r ExternalKubernetesResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*restclient.Config, error) {
	kubeconfig, err := getKubeconfig(ctx, r, tenantControlPlane)
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
		Timeout: time.Second * timeout,
	}

	return config, nil
}

func getTenantControllerInternalFQDN(tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", tenantControlPlane.GetName(), tenantControlPlane.GetNamespace())
}
