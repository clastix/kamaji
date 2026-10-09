// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"

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

type SACertificate struct {
	resource     *corev1.Secret
	Client       client.Client
	Name         string
	TmpDirectory string
}

func (r *SACertificate) GetHistogram() prometheus.Histogram {
	serviceaccountcertificateCollector = LazyLoadHistogramFromResource(serviceaccountcertificateCollector, r)

	return serviceaccountcertificateCollector
}

func (r *SACertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.SA.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Certificates.SA.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *SACertificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *SACertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *SACertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *SACertificate) GetClient() client.Client {
	return r.Client
}

func (r *SACertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *SACertificate) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *SACertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *SACertificate) GetName() string {
	return "sa-certificate"
}

func (r *SACertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.SA.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.SA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.SA.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

func (r *SACertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		isRotationRequested := utilities.IsRotationRequested(r.resource)

		if checksum := tenantControlPlane.Status.Certificates.SA.Checksum; !isRotationRequested && (len(checksum) > 0 && checksum == utilities.GetObjectChecksum(r.resource) || len(r.resource.UID) > 0) {
			isValid, err := crypto.CheckPublicAndPrivateKeyValidity(r.resource.Data[kubeadmconstants.ServiceAccountPublicKeyName], r.resource.Data[kubeadmconstants.ServiceAccountPrivateKeyName])
			if err != nil {
				logger.Info(fmt.Sprintf("%s public_key-private_key pair is not valid: %s", kubeadmconstants.ServiceAccountKeyBaseName, err.Error()))
			}
			if isValid {
				if utilities.IsPreviousKeyPruneRequested(r.resource) {
					r.resource.Data[kubeadmconstants.ServiceAccountPublicKeyName] = firstPEMBlock(r.resource.Data[kubeadmconstants.ServiceAccountPublicKeyName])
					utilities.RemovePreviousKeyPruneRequest(r.resource)
					utilities.SetObjectChecksum(r.resource, r.resource.Data)
				}

				return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
			}
		}

		config, err := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
		if err != nil {
			logger.Error(err, "cannot retrieve kubadm configuration")

			return err
		}

		sa, err := kubeadm.GeneratePublicKeyPrivateKeyPair(kubeadmconstants.ServiceAccountKeyBaseName, config)
		if err != nil {
			logger.Error(err, "cannot generate certificate and private key")

			return err
		}

		publicKey := sa.PublicKey
		// A requested rotation keeps the previous signing key as a verification-only key,
		// so tokens issued before the rotation are still accepted until they're refreshed.
		// Deleting the Secret is still the way to perform a hard rotation.
		if isRotationRequested && !utilities.IsPreviousKeyPruneRequested(r.resource) {
			publicKey = serviceAccountPublicKeyBundle(sa.PublicKey, r.resource.Data[kubeadmconstants.ServiceAccountPublicKeyName])
		}

		utilities.RemovePreviousKeyPruneRequest(r.resource)

		r.resource.Data = map[string][]byte{
			kubeadmconstants.ServiceAccountPublicKeyName:  publicKey,
			kubeadmconstants.ServiceAccountPrivateKeyName: sa.PrivateKey,
		}

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName())))

		if isRotationRequested {
			utilities.SetLastRotationTimestamp(r.resource)
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

// firstPEMBlock returns the first PEM block of the given content, which is the public key
// of the current signing key: any further block is a verification-only key.
func firstPEMBlock(content []byte) []byte {
	block, _ := pem.Decode(content)
	if block == nil {
		return content
	}

	return pem.EncodeToMemory(block)
}

// serviceAccountPublicKeyBundle returns the content of the sa.pub file upon a rotation:
// the new public key, followed by the public key of the previous signing key.
// Only the previous signing key is retained, older verification-only keys are dropped.
func serviceAccountPublicKeyBundle(newPublicKey, currentPublicKeys []byte) []byte {
	if block, _ := pem.Decode(currentPublicKeys); block == nil {
		return newPublicKey
	}

	signing, previous := firstPEMBlock(newPublicKey), firstPEMBlock(currentPublicKeys)
	if bytes.Equal(signing, previous) {
		return signing
	}

	return append(signing, previous...)
}
