// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"bytes"
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type CertificateResource struct {
	resource *corev1.Secret
	Client   client.Client
	Log      logr.Logger
	Name     string
}

func (r *CertificateResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.Certificate.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.ResourceVersion != r.resource.ResourceVersion
}

func (r *CertificateResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *CertificateResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *CertificateResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
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
		tenantControlPlane.Status.Addons.Konnectivity.Certificate.ResourceVersion = r.resource.ResourceVersion
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Certificate = kamajiv1alpha1.CertificatePrivateKeyPairStatus{}
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false

	return nil
}

func (r *CertificateResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		latestCARV := tenantControlPlane.Status.Certificates.CA.ResourceVersion
		actualCARV := r.resource.GetLabels()["latest-ca-rv"]
		if latestCARV == actualCARV {
			isValid, err := isCertificateAndKeyPairValid(
				r.resource.Data[corev1.TLSCertKey],
				r.resource.Data[corev1.TLSPrivateKeyKey],
			)
			if err != nil {
				r.Log.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", konnectivityCertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
		}

		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		secretCA := &corev1.Secret{}
		if err := r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			return err
		}

		ca := kubeadm.CertificatePrivateKeyPair{
			Name:        kubeadmconstants.CACertAndKeyBaseName,
			Certificate: secretCA.Data[kubeadmconstants.CACertName],
			PrivateKey:  secretCA.Data[kubeadmconstants.CAKeyName],
		}
		cert, privKey, err := getCertificateAndKeyPair(ca.Certificate, ca.PrivateKey)
		if err != nil {
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
				"latest-ca-rv":                latestCARV,
				"kamaji.clastix.io/name":      tenantControlPlane.GetName(),
				"kamaji.clastix.io/component": r.GetName(),
			},
		))

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func getCertificateAndKeyPair(caCert []byte, caPrivKey []byte) (*bytes.Buffer, *bytes.Buffer, error) {
	template := getCertTemplate()

	return crypto.GetCertificateAndKeyPair(template, caCert, caPrivKey)
}

func isCertificateAndKeyPairValid(cert []byte, privKey []byte) (bool, error) {
	return crypto.IsValidCertificateKeyPairBytes(cert, privKey)
}

func getCertTemplate() *x509.Certificate {
	serialNumber := big.NewInt(rand.Int63())

	return &x509.Certificate{
		PublicKeyAlgorithm: x509.RSA,
		SerialNumber:       serialNumber,
		Subject: pkix.Name{
			CommonName:   CertCommonName,
			Organization: []string{certOrganization},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(certExpirationDelayYears, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageCodeSigning,
		},
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
}
