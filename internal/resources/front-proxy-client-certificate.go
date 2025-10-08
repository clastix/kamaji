// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

type FrontProxyClientCertificate struct {
	resource                *corev1.Secret
	Client                  client.Client
	TmpDirectory            string
	CertExpirationThreshold time.Duration
}

func (r *FrontProxyClientCertificate) GetHistogram() prometheus.Histogram {
	frontproxycertificateCollector = LazyLoadHistogramFromResource(frontproxycertificateCollector, r)

	return frontproxycertificateCollector
}

func (r *FrontProxyClientCertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.FrontProxyClient.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *FrontProxyClientCertificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *FrontProxyClientCertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *FrontProxyClientCertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *FrontProxyClientCertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *FrontProxyClientCertificate) GetClient() client.Client {
	return r.Client
}

func (r *FrontProxyClientCertificate) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *FrontProxyClientCertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *FrontProxyClientCertificate) GetName() string {
	return "front-proxy-client-certificate"
}

func (r *FrontProxyClientCertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.FrontProxyClient.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.FrontProxyClient.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

// getCertificateDataFromSecret retrieves certificate data from the secret, trying multiple key formats
func (r *FrontProxyClientCertificate) getCertificateDataFromSecret(secretData map[string][]byte, preferredCertKey, preferredKeyKey string) (certData, keyData []byte, found bool) {
	// Try preferred keys first (from TenantControlPlane config)
	if cert, exists := secretData[preferredCertKey]; exists {
		if key, exists := secretData[preferredKeyKey]; exists {
			return cert, key, true
		}
	}

	// Try kubeadm format (front-proxy-client.crt/key)
	if cert, exists := secretData[kubeadmconstants.FrontProxyClientCertName]; exists {
		if key, exists := secretData[kubeadmconstants.FrontProxyClientKeyName]; exists {
			return cert, key, true
		}
	}

	// Try standard TLS format (tls.crt/tls.key)
	if cert, exists := secretData[corev1.TLSCertKey]; exists {
		if key, exists := secretData[corev1.TLSPrivateKeyKey]; exists {
			return cert, key, true
		}
	}

	return nil, nil, false
}

func (r *FrontProxyClientCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())
		// Retrieving the TenantControlPlane CA:
		// this is required to trigger a new generation in case of Certificate Authority rotation.
		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName}
		secretCA := &corev1.Secret{}
		if err := r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			logger.Error(err, "cannot retrieve CA secret")

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

		if checksum := tenantControlPlane.Status.Certificates.FrontProxyClient.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.GetObjectChecksum(r.resource) || len(r.resource.UID) > 0) {
			// Check existing generated certificate
			if certData, exists := r.resource.Data[kubeadmconstants.FrontProxyClientCertName]; exists {
				if keyData, exists := r.resource.Data[kubeadmconstants.FrontProxyClientKeyName]; exists {
					isCAValid, err := crypto.VerifyCertificate(certData, secretCA.Data[kubeadmconstants.FrontProxyCACertName], x509.ExtKeyUsageClientAuth)
					if err != nil {
						logger.Info(fmt.Sprintf("certificate-authority verify failed: %s", err.Error()))
					}

					isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
						certData,
						keyData,
						r.CertExpirationThreshold,
					)
					if err != nil {
						logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.FrontProxyClientCertAndKeyBaseName, err.Error()))
					}

					if isValid && isCAValid {
						return nil
					}
				}
			}
		}

		// Check if pregenerated Front Proxy Client certificate is specified
		if tenantControlPlane.Spec.PreGeneratedCertificates != nil && tenantControlPlane.Spec.PreGeneratedCertificates.FrontProxyClient != nil {
			logger.Info("Using pregenerated Front Proxy Client certificate")
			if err := r.usePreGeneratedFrontProxyClientCertificate(ctx, tenantControlPlane); err != nil {
				logger.Error(err, "cannot use pregenerated Front Proxy Client certificate")

				return err
			}
		} else {
			logger.Info("Generating new Front Proxy Client certificate")

			config, err := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
			if err != nil {
				logger.Error(err, "cannot retrieve kubeadm configuration")

				return err
			}

			ca := kubeadm.CertificatePrivateKeyPair{
				Name:        kubeadmconstants.FrontProxyCACertAndKeyBaseName,
				Certificate: secretCA.Data[kubeadmconstants.FrontProxyCACertName],
				PrivateKey:  secretCA.Data[kubeadmconstants.FrontProxyCAKeyName],
			}
			certificateKeyPair, err := kubeadm.GenerateCertificatePrivateKeyPair(kubeadmconstants.FrontProxyClientCertAndKeyBaseName, config, ca)
			if err != nil {
				logger.Error(err, "cannot generate certificate and private key")

				return err
			}

			r.resource.Data = map[string][]byte{
				kubeadmconstants.FrontProxyClientCertName: certificateKeyPair.Certificate,
				kubeadmconstants.FrontProxyClientKeyName:  certificateKeyPair.PrivateKey,
				// Add TLS keys for compatibility with external certificate management
				corev1.TLSCertKey:       certificateKeyPair.Certificate,
				corev1.TLSPrivateKeyKey: certificateKeyPair.PrivateKey,
			}
		}

		if isRotationRequested {
			utilities.SetLastRotationTimestamp(r.resource)
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return nil
	}
}

func (r *FrontProxyClientCertificate) usePreGeneratedFrontProxyClientCertificate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	certRef := tenantControlPlane.Spec.PreGeneratedCertificates.FrontProxyClient

	// Determine the namespace for the secret
	secretNamespace := certRef.SecretNamespace
	if secretNamespace == "" {
		secretNamespace = tenantControlPlane.GetNamespace()
	}

	// Get the referenced secret
	secret := &corev1.Secret{}
	secretKey := k8stypes.NamespacedName{
		Name:      certRef.SecretName,
		Namespace: secretNamespace,
	}

	if err := r.Client.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}

	// Determine certificate and private key keys
	certKey := certRef.CertificateKey
	if certKey == "" {
		certKey = corev1.TLSCertKey
	}

	privKeyKey := certRef.PrivateKeyKey
	if privKeyKey == "" {
		privKeyKey = corev1.TLSPrivateKeyKey
	}

	// Get certificate and private key data with flexible key reading
	certData, privKeyData, found := r.getCertificateDataFromSecret(secret.Data, certKey, privKeyKey)
	if !found {
		return fmt.Errorf("certificate and private key not found in secret %s (tried keys: %s/%s, %s/%s)",
			secretKey, certKey, privKeyKey, corev1.TLSCertKey, corev1.TLSPrivateKeyKey)
	}

	// Set the resource data using kubeadm constants and TLS keys for compatibility
	r.resource.Data = map[string][]byte{
		kubeadmconstants.FrontProxyClientCertName: certData,
		kubeadmconstants.FrontProxyClientKeyName:  privKeyData,
		// Add TLS keys for compatibility with external certificate management
		corev1.TLSCertKey:       certData,
		corev1.TLSPrivateKeyKey: privKeyData,
	}

	return nil
}
