// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type APIServerKubeletClientCertificate struct {
	resource     *corev1.Secret
	Client       client.Client
	TmpDirectory string
}

func (r *APIServerKubeletClientCertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.APIServerKubeletClient.Checksum != r.resource.GetAnnotations()[constants.Checksum]
}

func (r *APIServerKubeletClientCertificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *APIServerKubeletClientCertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *APIServerKubeletClientCertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *APIServerKubeletClientCertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
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
	return "api-server-kubelet-client-certificate"
}

func (r *APIServerKubeletClientCertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.APIServerKubeletClient.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.APIServerKubeletClient.Checksum = r.resource.GetAnnotations()[constants.Checksum]

	return nil
}

func (r *APIServerKubeletClientCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		if checksum := tenantControlPlane.Status.Certificates.APIServerKubeletClient.Checksum; len(checksum) > 0 && checksum == r.resource.GetAnnotations()[constants.Checksum] {
			isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
				r.resource.Data[kubeadmconstants.APIServerKubeletClientCertName],
				r.resource.Data[kubeadmconstants.APIServerKubeletClientKeyName],
			)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
		}

		config, err := getStoredKubeadmConfiguration(ctx, r, tenantControlPlane)
		if err != nil {
			logger.Error(err, "cannot retrieve kubeadm configuration")

			return err
		}

		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}

		secretCA := &corev1.Secret{}
		if err = r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			logger.Error(err, "cannot retrieve CA secret")

			return err
		}

		ca := kubeadm.CertificatePrivateKeyPair{
			Name:        kubeadmconstants.CACertAndKeyBaseName,
			Certificate: secretCA.Data[kubeadmconstants.CACertName],
			PrivateKey:  secretCA.Data[kubeadmconstants.CAKeyName],
		}
		certificateKeyPair, err := kubeadm.GenerateCertificatePrivateKeyPair(kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, config, ca)
		if err != nil {
			logger.Error(err, "cannot generate certificate and private key")

			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.APIServerKubeletClientCertName: certificateKeyPair.Certificate,
			kubeadmconstants.APIServerKubeletClientKeyName:  certificateKeyPair.PrivateKey,
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"kamaji.clastix.io/name":      tenantControlPlane.GetName(),
				"kamaji.clastix.io/component": r.GetName(),
			},
		))

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[constants.Checksum] = utilities.CalculateConfigMapChecksum(r.resource.StringData)
		r.resource.SetAnnotations(annotations)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
