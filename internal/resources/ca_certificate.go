// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type CACertificate struct {
	resource     *corev1.Secret
	isRotatingCA bool

	Client       client.Client
	TmpDirectory string
}

func (r *CACertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return r.isRotatingCA || tenantControlPlane.Status.Certificates.CA.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Certificates.CA.Checksum != r.resource.GetAnnotations()[constants.Checksum]
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
	tenantControlPlane.Status.Certificates.CA.Checksum = r.resource.GetAnnotations()[constants.Checksum]
	if r.isRotatingCA {
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionCARotating
	}

	return nil
}

func (r *CACertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		if checksum := tenantControlPlane.Status.Certificates.CA.Checksum; len(checksum) > 0 && checksum == r.resource.GetAnnotations()[constants.Checksum] {
			isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(
				r.resource.Data[kubeadmconstants.CACertName],
				r.resource.Data[kubeadmconstants.CAKeyName],
			)
			if err != nil {
				logger.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.CACertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
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
