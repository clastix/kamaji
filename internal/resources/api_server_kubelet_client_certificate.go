// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type APIServerKubeletClientCertificate struct {
	resource     *corev1.Secret
	Client       client.Client
	Log          logr.Logger
	Name         string
	TmpDirectory string
}

func (r *APIServerKubeletClientCertificate) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName != r.resource.GetName()
}

func (r *APIServerKubeletClientCertificate) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *APIServerKubeletClientCertificate) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *APIServerKubeletClientCertificate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *APIServerKubeletClientCertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *APIServerKubeletClientCertificate) GetClient() client.Client {
	return r.Client
}

func (r *APIServerKubeletClientCertificate) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *APIServerKubeletClientCertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (res controllerutil.OperationResult, err error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *APIServerKubeletClientCertificate) GetName() string {
	return r.Name
}

func (r *APIServerKubeletClientCertificate) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.APIServerKubeletClient.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName = r.resource.GetName()

	return nil
}

func (r *APIServerKubeletClientCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		latestConfigRV := getLatestConfigRV(*tenantControlPlane)
		actualConfigRV := r.resource.GetLabels()["latest-config-rv"]
		if latestConfigRV == actualConfigRV {
			isValid, err := kubeadm.IsCertificatePrivateKeyPairValid(
				r.resource.Data[kubeadmconstants.APIServerKubeletClientCertName],
				r.resource.Data[kubeadmconstants.APIServerKubeletClientKeyName],
			)
			if err != nil {
				r.Log.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
		}

		config, err := getStoredKubeadmConfiguration(ctx, r, tenantControlPlane)
		if err != nil {
			return err
		}

		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		secretCA := &corev1.Secret{}
		if err = r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			return err
		}

		ca := kubeadm.CertificatePrivateKeyPair{
			Name:        kubeadmconstants.CACertAndKeyBaseName,
			Certificate: secretCA.Data[kubeadmconstants.CACertName],
			PrivateKey:  secretCA.Data[kubeadmconstants.CAKeyName],
		}
		certificateKeyPair, err := kubeadm.GenerateCertificatePrivateKeyPair(kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, config, ca)
		if err != nil {
			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.APIServerKubeletClientCertName: certificateKeyPair.Certificate,
			kubeadmconstants.APIServerKubeletClientKeyName:  certificateKeyPair.PrivateKey,
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"latest-config-rv":            latestConfigRV,
				"kamaji.clastix.io/name":      tenantControlPlane.GetName(),
				"kamaji.clastix.io/component": r.GetName(),
			},
		))

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
