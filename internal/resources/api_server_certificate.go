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
	"github.com/clastix/kamaji/internal/webhook/handlers"
)

type APIServerCertificate struct {
	resource                *corev1.Secret
	Client                  client.Client
	TmpDirectory            string
	CertExpirationThreshold time.Duration
}

func (r *APIServerCertificate) GetHistogram() prometheus.Histogram {
	apiservercertificateCollector = LazyLoadHistogramFromResource(apiservercertificateCollector, r)

	return apiservercertificateCollector
}

func (r *APIServerCertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.APIServer.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *APIServerCertificate) ShouldCleanup(_ *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *APIServerCertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *APIServerCertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *APIServerCertificate) GetClient() client.Client {
	return r.Client
}

func (r *APIServerCertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *APIServerCertificate) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *APIServerCertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (res controllerutil.OperationResult, err error) {
	if err = (handlers.TenantControlPlaneCertSANs{}).ValidateCertSANs(tenantControlPlane); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *APIServerCertificate) GetName() string {
	return "api-server-certificate"
}

func (r *APIServerCertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.APIServer.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.APIServer.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.APIServer.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

func (r *APIServerCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())
		// The Kubeadm configuration must be retrieved in advance:
		// this is required to check also the certificate SAN
		config, kadmErr := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
		if kadmErr != nil {
			logger.Error(kadmErr, "cannot retrieve stored kubeadm configuration", "err", kadmErr.Error())

			return fmt.Errorf("failed to generate certificate and private key: %w", kadmErr)
		}
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

		if checksum := tenantControlPlane.Status.Certificates.APIServer.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.GetObjectChecksum(r.resource) || len(r.resource.UID) > 0) {
			isCAValid, err := crypto.VerifyCertificate(r.resource.Data[kubeadmconstants.APIServerCertName], secretCA.Data[kubeadmconstants.CACertName], x509.ExtKeyUsageServerAuth)
			if err != nil {
				logger.Info(fmt.Sprintf("certificate-authority verify failed: %s", err.Error()))
			}

			isCertValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
				r.resource.Data[kubeadmconstants.APIServerCertName],
				r.resource.Data[kubeadmconstants.APIServerKeyName],
				r.CertExpirationThreshold,
			)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.APIServerCertAndKeyBaseName, err.Error()))
			}

			commonNames := config.InitConfiguration.APIServer.CertSANs

			addr, _, aErr := tenantControlPlane.AssignedControlPlaneAddress()
			if aErr == nil {
				commonNames = append(commonNames, addr)
			}

			dnsNamesMatches, dnsErr := crypto.CheckCertificateNamesAndIPs(r.resource.Data[kubeadmconstants.APIServerCertName], commonNames)
			if dnsErr != nil {
				logger.Info(fmt.Sprintf("%s SAN check returned an error: %s", kubeadmconstants.APIServerCertAndKeyBaseName, err.Error()))
			}

			if isCAValid && isCertValid && dnsNamesMatches {
				return nil
			}
		}

		// Check if pregenerated API Server certificate is specified
		if tenantControlPlane.Spec.PreGeneratedCertificates != nil && tenantControlPlane.Spec.PreGeneratedCertificates.APIServer != nil {
			logger.Info("Using pregenerated API Server certificate")
			if err := r.usePreGeneratedAPIServerCertificate(ctx, tenantControlPlane); err != nil {
				logger.Error(err, "cannot use pregenerated API Server certificate")

				return err
			}
		} else {
			logger.Info("Generating new API Server certificate")

			// Ensure the assigned control plane address is included in the certificate SANs
			addr, _, addrErr := tenantControlPlane.AssignedControlPlaneAddress()
			if addrErr == nil && addr != "" {
				// Check if address is already in CertSANs
				hasAddr := false
				for _, san := range config.InitConfiguration.APIServer.CertSANs {
					if san == addr {
						hasAddr = true
						break
					}
				}
				if !hasAddr {
					config.InitConfiguration.APIServer.CertSANs = append(config.InitConfiguration.APIServer.CertSANs, addr)
				}
			}

			ca := kubeadm.CertificatePrivateKeyPair{
				Name:        kubeadmconstants.CACertAndKeyBaseName,
				Certificate: secretCA.Data[kubeadmconstants.CACertName],
				PrivateKey:  secretCA.Data[kubeadmconstants.CAKeyName],
			}
			certificateKeyPair, err := kubeadm.GenerateCertificatePrivateKeyPair(kubeadmconstants.APIServerCertAndKeyBaseName, config, ca)
			if err != nil {
				logger.Error(err, "cannot generate certificate and private key")

				return err
			}

			r.resource.Data = map[string][]byte{
				kubeadmconstants.APIServerCertName: certificateKeyPair.Certificate,
				kubeadmconstants.APIServerKeyName:  certificateKeyPair.PrivateKey,
			}
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return nil
	}
}

func (r *APIServerCertificate) usePreGeneratedAPIServerCertificate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	certRef := tenantControlPlane.Spec.PreGeneratedCertificates.APIServer

	// Secrets must be in the same namespace as the TenantControlPlane
	secretKey := k8stypes.NamespacedName{
		Name:      certRef.SecretName,
		Namespace: tenantControlPlane.GetNamespace(),
	}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}

	certKey := certRef.CertificateKey
	if certKey == "" {
		certKey = corev1.TLSCertKey
	}

	privKeyKey := certRef.PrivateKeyKey
	if privKeyKey == "" {
		privKeyKey = corev1.TLSPrivateKeyKey
	}

	certData, exists := secret.Data[certKey]
	if !exists {
		return fmt.Errorf("certificate key %s not found in secret %s", certKey, secretKey)
	}

	privKeyData, exists := secret.Data[privKeyKey]
	if !exists {
		return fmt.Errorf("private key %s not found in secret %s", privKeyKey, secretKey)
	}

	r.resource.Data = map[string][]byte{
		kubeadmconstants.APIServerCertName: certData,
		kubeadmconstants.APIServerKeyName:  privKeyData,
	}

	return nil
}
