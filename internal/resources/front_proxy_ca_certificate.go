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

type FrontProxyCACertificate struct {
	resource     *corev1.Secret
	Client       client.Client
	Log          logr.Logger
	Name         string
	TmpDirectory string
}

func (r *FrontProxyCACertificate) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Certificates.FrontProxyCA.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *FrontProxyCACertificate) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *FrontProxyCACertificate) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *FrontProxyCACertificate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	r.Name = "front-proxy-ca-certificate"

	return nil
}

func (r *FrontProxyCACertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
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
	return r.Name
}

func (r *FrontProxyCACertificate) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Certificates.FrontProxyCA.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.FrontProxyCA.Checksum = r.resource.GetAnnotations()["checksum"]

	return nil
}

func (r *FrontProxyCACertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if checksum := tenantControlPlane.Status.Certificates.FrontProxyCA.Checksum; len(checksum) > 0 && checksum == r.resource.GetAnnotations()["checksum"] {
			isValid, err := kubeadm.IsCertificatePrivateKeyPairValid(
				r.resource.Data[kubeadmconstants.FrontProxyCACertName],
				r.resource.Data[kubeadmconstants.FrontProxyCAKeyName],
			)
			if err != nil {
				r.Log.Info(fmt.Sprintf("%s certificate-private_key pair is not valid: %s", kubeadmconstants.FrontProxyCACertAndKeyBaseName, err.Error()))
			}
			if isValid {
				return nil
			}
		}

		config, err := getStoredKubeadmConfiguration(ctx, r, tenantControlPlane)
		if err != nil {
			return err
		}

		ca, err := kubeadm.GenerateCACertificatePrivateKeyPair(kubeadmconstants.FrontProxyCACertAndKeyBaseName, config)
		if err != nil {
			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.FrontProxyCACertName: ca.Certificate,
			kubeadmconstants.FrontProxyCAKeyName:  ca.PrivateKey,
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
