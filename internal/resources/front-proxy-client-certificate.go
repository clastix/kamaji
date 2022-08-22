// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type FrontProxyClientCertificate struct {
	resource     *corev1.Secret
	Client       client.Client
	Log          logr.Logger
	Name         string
	TmpDirectory string
}

func (r *FrontProxyClientCertificate) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.FrontProxyClient.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *FrontProxyClientCertificate) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *FrontProxyClientCertificate) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *FrontProxyClientCertificate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *FrontProxyClientCertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
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
	return r.Name
}

func (r *FrontProxyClientCertificate) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.FrontProxyClient.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.FrontProxyClient.Checksum = r.resource.GetAnnotations()["checksum"]

	return nil
}

func (r *FrontProxyClientCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if checksum := tenantControlPlane.Status.Certificates.FrontProxyClient.Checksum; len(checksum) > 0 && checksum == r.resource.GetAnnotations()["checksum"] {
			isValid, err := kubeadm.IsCertificatePrivateKeyPairValid(
				r.resource.Data[kubeadmconstants.FrontProxyClientCertName],
				r.resource.Data[kubeadmconstants.FrontProxyClientKeyName],
			)
			if err != nil {
				r.Log.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.FrontProxyClientCertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
		}

		config, err := getStoredKubeadmConfiguration(ctx, r, tenantControlPlane)
		if err != nil {
			return err
		}

		namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName}
		secretCA := &corev1.Secret{}
		if err = r.Client.Get(ctx, namespacedName, secretCA); err != nil {
			return err
		}

		ca := kubeadm.CertificatePrivateKeyPair{
			Name:        kubeadmconstants.FrontProxyCACertAndKeyBaseName,
			Certificate: secretCA.Data[kubeadmconstants.FrontProxyCACertName],
			PrivateKey:  secretCA.Data[kubeadmconstants.FrontProxyCAKeyName],
		}
		certificateKeyPair, err := kubeadm.GenerateCertificatePrivateKeyPair(kubeadmconstants.FrontProxyClientCertAndKeyBaseName, config, ca)
		if err != nil {
			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.FrontProxyClientCertName: certificateKeyPair.Certificate,
			kubeadmconstants.FrontProxyClientKeyName:  certificateKeyPair.PrivateKey,
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
		annotations["checksum"] = utilities.CalculateConfigMapChecksum(r.resource.StringData)
		r.resource.SetAnnotations(annotations)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
