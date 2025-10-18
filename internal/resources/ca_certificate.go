// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type CACertificate struct {
	resource     *corev1.Secret
	isRotatingCA bool

	Client                  client.Client
	TmpDirectory            string
	CertExpirationThreshold time.Duration
}

func (r *CACertificate) GetHistogram() prometheus.Histogram {
	certificateauthorityCollector = LazyLoadHistogramFromResource(certificateauthorityCollector, r)

	return certificateauthorityCollector
}

func (r *CACertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return r.isRotatingCA || tenantControlPlane.Status.Certificates.CA.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Certificates.CA.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *CACertificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *CACertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *CACertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *CACertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *CACertificate) GetClient() client.Client {
	return r.Client
}

func (r *CACertificate) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *CACertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *CACertificate) GetName() string {
	return "ca"
}

func (r *CACertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.CA.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.CA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.CA.Checksum = utilities.GetObjectChecksum(r.resource)
	if r.isRotatingCA {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionCARotating
	}

	return nil
}

func (r *CACertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		isRotationRequested := utilities.IsRotationRequested(r.resource)

		if checksum := tenantControlPlane.Status.Certificates.CA.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.GetObjectChecksum(r.resource) || len(r.resource.UID) > 0) {
			isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
				r.resource.Data[kubeadmconstants.CACertName],
				r.resource.Data[kubeadmconstants.CAKeyName],
				r.CertExpirationThreshold,
			)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.CACertAndKeyBaseName, err.Error()))
			}
			// Appending the Cluster API required keys if they're missing:
			// with this we're sure to avoid introducing breaking changes.
			if isValid && (!bytes.Equal(r.resource.Data[corev1.TLSCertKey], r.resource.Data[kubeadmconstants.CACertName]) || !bytes.Equal(r.resource.Data[kubeadmconstants.CAKeyName], r.resource.Data[corev1.TLSPrivateKeyKey])) {
				r.resource.Data[corev1.TLSCertKey] = r.resource.Data[kubeadmconstants.CACertName]
				r.resource.Data[corev1.TLSPrivateKeyKey] = r.resource.Data[kubeadmconstants.CAKeyName]
			}

			if isValid {
				return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
			}
		}

		if isRotationRequested {
			utilities.SetLastRotationTimestamp(r.resource)
		}

		if tenantControlPlane.Status.Kubernetes.Version.Status != nil && *tenantControlPlane.Status.Kubernetes.Version.Status != kamajiv1alpha1.VersionProvisioning {
			r.isRotatingCA = true
		}

		config, err := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
		if err != nil {
			logger.Error(err, "cannot retrieve kubeadm configuration")

			return err
		}

		ca, err := kubeadm.GenerateCACertificatePrivateKeyPair(kubeadmconstants.CACertAndKeyBaseName, config)
		if err != nil {
			logger.Error(err, "cannot generate certificate and private key")

			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.CACertName: ca.Certificate,
			kubeadmconstants.CAKeyName:  ca.PrivateKey,
			// Required for Cluster API integration which is reading the basic TLS keys.
			// We cannot switch over basic corev1.Secret keys for backward compatibility,
			// it would require a new CA generation breaking all the clusters deployed.
			corev1.TLSCertKey:       ca.Certificate,
			corev1.TLSPrivateKeyKey: ca.PrivateKey,
		}

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName())))

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
