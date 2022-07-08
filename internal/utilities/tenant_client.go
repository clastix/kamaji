// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
)

const (
	kubeadmConfigSecretKeyName = "admin.conf"
)

func GetTenantClient(ctx context.Context, c client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (client.Client, error) {
	options := client.Options{}
	config, err := getRESTClientConfig(ctx, c, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return client.New(config, options)
}

func GetTenantRESTClient(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*clientset.Clientset, error) {
	config, err := getRESTClientConfig(ctx, client, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(config)
}

func GetKubeconfigSecret(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	if err := client.Get(ctx, k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.KubeConfig.Admin.SecretName}, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func getRESTClientConfig(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*restclient.Config, error) {
	kubeconfig, err := GetKubeconfig(ctx, client, tenantControlPlane)
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
		Timeout: 10 * time.Second,
	}

	return config, nil
}

func GetKubeconfig(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeconfigutil.Kubeconfig, error) {
	secretKubeconfig, err := GetKubeconfigSecret(ctx, client, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	bytes, ok := secretKubeconfig.Data[kubeadmConfigSecretKeyName]
	if !ok {
		return nil, fmt.Errorf("%s is not into kubeconfig secret", kubeadmConfigSecretKeyName)
	}

	return kubeconfigutil.GetKubeconfigFromBytes(bytes)
}

func getTenantControllerInternalFQDN(tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", tenantControlPlane.GetName(), tenantControlPlane.GetNamespace())
}
