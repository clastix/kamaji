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

type CertificateResource struct {
	resource *corev1.Secret
	Client   client.Client
	Name     string
}

func (r *CertificateResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.Certificate.Checksum != r.resource.GetAnnotations()[constants.Checksum]
}

func (r *CertificateResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *CertificateResource) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot delete the required resource")

			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *CertificateResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *CertificateResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *CertificateResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *CertificateResource) GetName() string {
	return r.Name
}

func (r *CertificateResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.LastUpdate = metav1.Now()
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.SecretName = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.Checksum = r.resource.GetAnnotations()[constants.Checksum]
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Certificate = kamajiv1alpha1.CertificatePrivateKeyPairStatus{}
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false

	return nil
}

func (r *CertificateResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		if checksum := tenantControlPlane.Status.Addons.Konnectivity.Certificate.Checksum; len(checksum) > 0 && checksum == utilities.CalculateMapChecksum(r.resource.Data) {
			isValid, err := crypto.IsValidCertificateKeyPairBytes(r.resource.Data[corev1.TLSCertKey], r.resource.Data[corev1.TLSPrivateKeyKey])
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", konnectivityCertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
		}

		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		secretCA := &corev1.Secret{}
		if err := r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			logger.Error(err, "cannot retrieve the CA secret")

			return err
		}

		ca := kubeadm.CertificatePrivateKeyPair{
			Name:        kubeadmconstants.CACertAndKeyBaseName,
			Certificate: secretCA.Data[kubeadmconstants.CACertName],
			PrivateKey:  secretCA.Data[kubeadmconstants.CAKeyName],
		}

		cert, privKey, err := crypto.GenerateCertificatePrivateKeyPair(crypto.NewCertificateTemplate(CertCommonName), ca.Certificate, ca.PrivateKey)
		if err != nil {
			logger.Error(err, "unable to generate certificate and private key")

			return err
		}

		r.resource.Type = corev1.SecretTypeTLS
		r.resource.Data = map[string][]byte{
			corev1.TLSCertKey:       cert.Bytes(),
			corev1.TLSPrivateKeyKey: privKey.Bytes(),
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
		annotations[constants.Checksum] = utilities.CalculateMapChecksum(r.resource.Data)
		r.resource.SetAnnotations(annotations)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
