// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type CACertificate struct {
	resource     *corev1.Secret
	Client       client.Client
	Log          logr.Logger
	Name         string
	TmpDirectory string
}

func (r *CACertificate) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.CA.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Certificates.CA.ResourceVersion != r.resource.ResourceVersion
}

func (r *CACertificate) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *CACertificate) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *CACertificate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *CACertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
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
	return r.Name
}

func (r *CACertificate) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.CA.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.CA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.CA.ResourceVersion = r.resource.ResourceVersion

	return nil
}

func (r *CACertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		isValid, err := kubeadm.IsCertificatePrivateKeyPairValid(
			r.resource.Data[kubeadmconstants.CACertName],
			r.resource.Data[kubeadmconstants.CAKeyName],
		)
		if err != nil {
			r.Log.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.CACertAndKeyBaseName, err.Error()))
		}
		if isValid {
			return nil
		}

		config, _, err := getKubeadmConfiguration(ctx, r, tenantControlPlane)
		if err != nil {
			return err
		}

		ca, err := kubeadm.GenerateCACertificatePrivateKeyPair(kubeadmconstants.CACertAndKeyBaseName, config)
		if err != nil {
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

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
