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
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func GetTenantClient(ctx context.Context, c client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (client.Client, error) {
	options := client.Options{}
	config, err := GetRESTClientConfig(ctx, c, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return client.New(config, options)
}

func GetTenantClientSet(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*clientset.Clientset, error) {
	config, err := GetRESTClientConfig(ctx, client, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(config)
}

func GetTenantKubeconfig(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*clientcmdapiv1.Config, error) {
	secretKubeconfig := &corev1.Secret{}
	if err := client.Get(ctx, k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.KubeConfig.Admin.SecretName}, secretKubeconfig); err != nil {
		return nil, err
	}

	secretKey := kubeadmconstants.SuperAdminKubeConfigFileName
	v, ok := tenantControlPlane.GetAnnotations()[kamajiv1alpha1.KubeconfigSecretKeyAnnotation]
	if ok && v != "" {
		secretKey = v
	}

	return DecodeKubeconfig(*secretKubeconfig, secretKey)
}

func GetRESTClientConfig(ctx context.Context, client client.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*restclient.Config, error) {
	kubeconfig, err := GetTenantKubeconfig(ctx, client, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	config := &restclient.Config{
		Host: fmt.Sprintf("https://%s.%s.svc.cluster.local:%d", tenantControlPlane.GetName(), tenantControlPlane.GetNamespace(), tenantControlPlane.Spec.NetworkProfile.Port),
		TLSClientConfig: restclient.TLSClientConfig{
			CAData:   kubeconfig.Clusters[0].Cluster.CertificateAuthorityData,
			CertData: kubeconfig.AuthInfos[0].AuthInfo.ClientCertificateData,
			KeyData:  kubeconfig.AuthInfos[0].AuthInfo.ClientKeyData,
		},
		Timeout: 10 * time.Second,
	}

	return config, nil
}
