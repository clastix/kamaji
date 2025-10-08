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

type APIServerKubeletClientCertificate struct {
	resource                *corev1.Secret
	Client                  client.Client
	TmpDirectory            string
	CertExpirationThreshold time.Duration
}

func (r *APIServerKubeletClientCertificate) GetHistogram() prometheus.Histogram {
	clientcertificateCollector = LazyLoadHistogramFromResource(clientcertificateCollector, r)

	return clientcertificateCollector
}

func (r *APIServerKubeletClientCertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.APIServerKubeletClient.Checksum != utilities.GetObjectChecksum(r.resource)
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
	tenantControlPlane.Status.Certificates.APIServerKubeletClient.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

func (r *APIServerKubeletClientCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())
		// Retrieving the TenantControlPlane CA:
		// this is required to trigger a new generation in case of Certificate Authority rotation.
		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
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

		if checksum := tenantControlPlane.Status.Certificates.APIServerKubeletClient.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.GetObjectChecksum(r.resource) || len(r.resource.UID) > 0) {
			isCAValid, err := crypto.VerifyCertificate(r.resource.Data[kubeadmconstants.APIServerKubeletClientCertName], secretCA.Data[kubeadmconstants.CACertName], x509.ExtKeyUsageClientAuth)
			if err != nil {
				logger.Info(fmt.Sprintf("certificate-authority verify failed: %s", err.Error()))
			}

			isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
				r.resource.Data[kubeadmconstants.APIServerKubeletClientCertName],
				r.resource.Data[kubeadmconstants.APIServerKubeletClientKeyName],
				r.CertExpirationThreshold,
			)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, err.Error()))
			}

			if isValid && isCAValid {
				return nil
			}
		}

		// Check if pregenerated Kubelet client certificate is specified
		if tenantControlPlane.Spec.PreGeneratedCertificates != nil && tenantControlPlane.Spec.PreGeneratedCertificates.KubeletClient != nil {
			logger.Info("Using pregenerated Kubelet client certificate")
			if err := r.usePreGeneratedKubeletClientCertificate(ctx, tenantControlPlane); err != nil {
				logger.Error(err, "cannot use pregenerated Kubelet client certificate")

				return err
			}
		} else {
			logger.Info("Generating new Kubelet client certificate")

			config, err := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
			if err != nil {
				logger.Error(err, "cannot retrieve kubeadm configuration")

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

			if isRotationRequested {
				utilities.SetLastRotationTimestamp(r.resource)
			}

			r.resource.Data = map[string][]byte{
				kubeadmconstants.APIServerKubeletClientCertName: certificateKeyPair.Certificate,
				kubeadmconstants.APIServerKubeletClientKeyName:  certificateKeyPair.PrivateKey,
				// Add TLS keys for compatibility with external certificate management
				corev1.TLSCertKey:       certificateKeyPair.Certificate,
				corev1.TLSPrivateKeyKey: certificateKeyPair.PrivateKey,
			}
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return nil
	}
}

func (r *APIServerKubeletClientCertificate) usePreGeneratedKubeletClientCertificate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	certRef := tenantControlPlane.Spec.PreGeneratedCertificates.KubeletClient

	// Determine the namespace for the secret
	secretNamespace := certRef.SecretNamespace
	if secretNamespace == "" {
		secretNamespace = tenantControlPlane.GetNamespace()
	}

	// Fetch the pregenerated certificate secret
	pregenSecret := &corev1.Secret{}
	err := r.Client.Get(ctx, k8stypes.NamespacedName{
		Name:      certRef.SecretName,
		Namespace: secretNamespace,
	}, pregenSecret)
	if err != nil {
		return fmt.Errorf("failed to get pregenerated Kubelet client certificate secret %s/%s: %w", secretNamespace, certRef.SecretName, err)
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

	// Get certificate data with fallback logic - try kubeadm format first, then TLS format
	certData, exists := pregenSecret.Data[kubeadmconstants.APIServerKubeletClientCertName]
	if !exists {
		// Fallback to configured certificate key (usually tls.crt)
		if fallbackCertData, fallbackExists := pregenSecret.Data[certKey]; fallbackExists {
			certData = fallbackCertData
		} else {
			return fmt.Errorf("certificate key %s not found in secret %s/%s, and fallback key %s also not found", kubeadmconstants.APIServerKubeletClientCertName, secretNamespace, certRef.SecretName, certKey)
		}
	}

	// Get private key data with fallback logic - try kubeadm format first, then TLS format
	privKeyData, exists := pregenSecret.Data[kubeadmconstants.APIServerKubeletClientKeyName]
	if !exists {
		// Fallback to configured private key (usually tls.key)
		if fallbackPrivData, fallbackExists := pregenSecret.Data[privKeyKey]; fallbackExists {
			privKeyData = fallbackPrivData
		} else {
			return fmt.Errorf("private key %s not found in secret %s/%s, and fallback key %s also not found", kubeadmconstants.APIServerKubeletClientKeyName, secretNamespace, certRef.SecretName, privKeyKey)
		}
	}

	// Validate certificate and key format
	isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(certData, privKeyData, 0)
	if err != nil {
		return fmt.Errorf("invalid certificate or private key format: %w", err)
	}
	if !isValid {
		return fmt.Errorf("certificate and private key pair validation failed")
	}

	// Set the resource data with both kubeadm and TLS keys for compatibility
	r.resource.Data = map[string][]byte{
		kubeadmconstants.APIServerKubeletClientCertName: certData,
		kubeadmconstants.APIServerKubeletClientKeyName:  privKeyData,
		// Add TLS keys for compatibility with external certificate management
		corev1.TLSCertKey:       certData,
		corev1.TLSPrivateKeyKey: privKeyData,
	}

	return nil
}
