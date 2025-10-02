// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type CertificateResource struct {
	resource                *corev1.Secret
	Client                  client.Client
	CertExpirationThreshold time.Duration
}

func (r *CertificateResource) GetHistogram() prometheus.Histogram {
	certificateCollector = resources.LazyLoadHistogramFromResource(certificateCollector, r)

	return certificateCollector
}

func (r *CertificateResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.Certificate.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *CertificateResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil && tenantControlPlane.Status.Addons.Konnectivity.Enabled
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
			Name:      utilities.AddTenantPrefix(r.GetName(), tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *CertificateResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tenantControlPlane.Spec.Addons.Konnectivity == nil {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *CertificateResource) GetName() string {
	return "konnectivity-certificate"
}

func (r *CertificateResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Addons.Konnectivity.Certificate = kamajiv1alpha1.CertificatePrivateKeyPairStatus{}

	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.LastUpdate = metav1.Now()
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.SecretName = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.Checksum = utilities.GetObjectChecksum(r.resource)
	}

	return nil
}

func (r *CertificateResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		// Retrieving the TenantControlPlane CA:
		// this is required to trigger a new generation in case of Certificate Authority rotation.
		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		secretCA := &corev1.Secret{}
		if err := r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			logger.Error(err, "cannot retrieve the CA secret")

			return err
		}

		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()),
			map[string]string{
				constants.ControllerLabelResource: utilities.CertificateX509Label,
			},
		))

		if err := ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme()); err != nil {
			logger.Error(err, "cannot set controller reference", "resource", r.GetName())

			return err
		}

		isRotationRequested := utilities.IsRotationRequested(r.resource)

		if checksum := tenantControlPlane.Status.Addons.Konnectivity.Certificate.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.CalculateMapChecksum(r.resource.Data)) {
			isCAValid, err := crypto.VerifyCertificate(r.resource.Data[corev1.TLSCertKey], secretCA.Data[kubeadmconstants.CACertName], x509.ExtKeyUsageServerAuth)
			if err != nil {
				logger.Info(fmt.Sprintf("certificate-authority verify failed: %s", err.Error()))
			}

			isValid, err := crypto.IsValidCertificateKeyPairBytes(r.resource.Data[corev1.TLSCertKey], r.resource.Data[corev1.TLSPrivateKeyKey], r.CertExpirationThreshold)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", konnectivityCertAndKeyBaseName, err.Error()))
			}
			if isCAValid && isValid {
				return nil
			}
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

		if isRotationRequested {
			utilities.SetLastRotationTimestamp(r.resource)
		}

		r.resource.Type = corev1.SecretTypeTLS
		r.resource.Data = map[string][]byte{
			corev1.TLSCertKey:       cert.Bytes(),
			corev1.TLSPrivateKeyKey: privKey.Bytes(),
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return nil
	}
}
