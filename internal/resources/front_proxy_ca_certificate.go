// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
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
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type FrontProxyCACertificate struct {
	resource                *corev1.Secret
	Client                  client.Client
	TmpDirectory            string
	CertExpirationThreshold time.Duration
}

func (r *FrontProxyCACertificate) GetHistogram() prometheus.Histogram {
	frontproxycaCollector = LazyLoadHistogramFromResource(frontproxycaCollector, r)

	return frontproxycaCollector
}

func (r *FrontProxyCACertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.FrontProxyCA.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *FrontProxyCACertificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *FrontProxyCACertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *FrontProxyCACertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *FrontProxyCACertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *FrontProxyCACertificate) GetClient() client.Client {
	return r.Client
}

func (r *FrontProxyCACertificate) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *FrontProxyCACertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *FrontProxyCACertificate) GetName() string {
	return "front-proxy-ca-certificate"
}

func (r *FrontProxyCACertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.FrontProxyCA.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.FrontProxyCA.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

func (r *FrontProxyCACertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		isRotationRequested := utilities.IsRotationRequested(r.resource)

		if checksum := tenantControlPlane.Status.Certificates.FrontProxyCA.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.GetObjectChecksum(r.resource) || len(r.resource.UID) > 0) {
			isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
				r.resource.Data[kubeadmconstants.FrontProxyCACertName],
				r.resource.Data[kubeadmconstants.FrontProxyCAKeyName],
				r.CertExpirationThreshold,
			)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.FrontProxyCACertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
			}
		}

		// Check if pregenerated Front Proxy CA certificate is specified
		if tenantControlPlane.Spec.PreGeneratedCertificates != nil && tenantControlPlane.Spec.PreGeneratedCertificates.FrontProxyCA != nil {
			logger.Info("Using pregenerated Front Proxy CA certificate")
			if err := r.usePreGeneratedFrontProxyCACertificate(ctx, tenantControlPlane); err != nil {
				logger.Error(err, "cannot use pregenerated Front Proxy CA certificate")

				return err
			}
		} else {
			logger.Info("Generating new Front Proxy CA certificate")

			config, err := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
			if err != nil {
				logger.Error(err, "cannot retrieve kubeadm configuration")

				return err
			}

			ca, err := kubeadm.GenerateCACertificatePrivateKeyPair(kubeadmconstants.FrontProxyCACertAndKeyBaseName, config)
			if err != nil {
				logger.Error(err, "cannot generate certificate and private key")

				return err
			}

			r.resource.Data = map[string][]byte{
				kubeadmconstants.FrontProxyCACertName: ca.Certificate,
				kubeadmconstants.FrontProxyCAKeyName:  ca.PrivateKey,
				// Add TLS keys for compatibility with external certificate management
				corev1.TLSCertKey:       ca.Certificate,
				corev1.TLSPrivateKeyKey: ca.PrivateKey,
			}
		}

		r.resource.SetLabels(utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()))

		if isRotationRequested {
			utilities.SetLastRotationTimestamp(r.resource)
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *FrontProxyCACertificate) usePreGeneratedFrontProxyCACertificate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	certRef := tenantControlPlane.Spec.PreGeneratedCertificates.FrontProxyCA

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

	// Get certificate data with fallback logic - try kubeadm format first, then TLS format
	certData, exists := secret.Data[kubeadmconstants.FrontProxyCACertName]
	if !exists {
		// Fallback to configured certificate key (usually tls.crt)
		if fallbackCertData, fallbackExists := secret.Data[certKey]; fallbackExists {
			certData = fallbackCertData
		} else {
			return fmt.Errorf("certificate key %s not found in secret %s, and fallback key %s also not found", kubeadmconstants.FrontProxyCACertName, secretKey, certKey)
		}
	}

	// Get private key data with fallback logic - try kubeadm format first, then TLS format
	privKeyData, exists := secret.Data[kubeadmconstants.FrontProxyCAKeyName]
	if !exists {
		// Fallback to configured private key (usually tls.key)
		if fallbackPrivData, fallbackExists := secret.Data[privKeyKey]; fallbackExists {
			privKeyData = fallbackPrivData
		} else {
			return fmt.Errorf("private key %s not found in secret %s, and fallback key %s also not found", kubeadmconstants.FrontProxyCAKeyName, secretKey, privKeyKey)
		}
	}

	// Set the resource data using kubeadm constants and TLS keys for compatibility
	r.resource.Data = map[string][]byte{
		kubeadmconstants.FrontProxyCACertName: certData,
		kubeadmconstants.FrontProxyCAKeyName:  privKeyData,
		// Add TLS keys for compatibility with external certificate management
		corev1.TLSCertKey:       certData,
		corev1.TLSPrivateKeyKey: privKeyData,
	}

	return nil
}
