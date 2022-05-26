// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeconfigResource struct {
	resource *corev1.Secret
	Client   client.Client
	Name     string
}

func (r *KubeconfigResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.Kubeconfig.SecretName != r.resource.GetName()
}

func (r *KubeconfigResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *KubeconfigResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *KubeconfigResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubeconfigResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *KubeconfigResource) GetClient() client.Client {
	return r.Client
}

func (r *KubeconfigResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubeconfigResource) GetName() string {
	return r.Name
}

func (r *KubeconfigResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Kubeconfig.LastUpdate = metav1.Now()
		tenantControlPlane.Status.Addons.Konnectivity.Kubeconfig.SecretName = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false
	tenantControlPlane.Status.Addons.Konnectivity.Kubeconfig = kamajiv1alpha1.KubeconfigStatus{}

	return nil
}

func (r *KubeconfigResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		latestCARV := tenantControlPlane.Status.Addons.Konnectivity.Certificate.ResourceVersion
		actualCARV := r.resource.GetLabels()["latest-certificate-rv"]
		if latestCARV == actualCARV {
			return nil
		}

		caNamespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		secretCA := &corev1.Secret{}
		if err := r.Client.Get(ctx, caNamespacedName, secretCA); err != nil {
			return err
		}

		certificateNamespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Addons.Konnectivity.Certificate.SecretName}
		secretCertificate := &corev1.Secret{}
		if err := r.Client.Get(ctx, certificateNamespacedName, secretCertificate); err != nil {
			return err
		}

		userName := CertCommonName
		clusterName := defaultClusterName
		contextName := fmt.Sprintf("%s@%s", userName, clusterName)

		kubeconfig := &clientcmdapiv1.Config{
			Kind:       "Config",
			APIVersion: kubeconfigAPIVersion,
			AuthInfos: []clientcmdapiv1.NamedAuthInfo{
				{
					Name: userName,
					AuthInfo: clientcmdapiv1.AuthInfo{
						ClientKeyData:         secretCertificate.Data[corev1.TLSPrivateKeyKey],
						ClientCertificateData: secretCertificate.Data[corev1.TLSCertKey],
					},
				},
			},
			Clusters: []clientcmdapiv1.NamedCluster{
				{
					Name: clusterName,
					Cluster: clientcmdapiv1.Cluster{
						Server:                   r.getServer(*tenantControlPlane),
						CertificateAuthorityData: secretCA.Data[kubeadmconstants.CACertName],
					},
				},
			},
			Contexts: []clientcmdapiv1.NamedContext{
				{
					Name: contextName,
					Context: clientcmdapiv1.Context{
						Cluster:  clusterName,
						AuthInfo: userName,
					},
				},
			},
			CurrentContext: contextName,
		}

		kubeconfigBytes, err := utilities.EncondeToYaml(kubeconfig)
		if err != nil {
			return err
		}

		r.resource.Data = map[string][]byte{
			konnectivityKubeconfigFileName: kubeconfigBytes,
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"latest-certificate-rv":       latestCARV,
				"kamaji.clastix.io/name":      tenantControlPlane.GetName(),
				"kamaji.clastix.io/component": r.GetName(),
			},
		))

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubeconfigResource) getServer(tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("https://%s:%d", "localhost", tenantControlPlane.Spec.NetworkProfile.Port)
}
