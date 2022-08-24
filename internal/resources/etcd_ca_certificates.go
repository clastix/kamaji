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
	"github.com/clastix/kamaji/internal/etcd"
	"github.com/clastix/kamaji/internal/utilities"
)

type ETCDCACertificatesResource struct {
	resource  *corev1.Secret
	Client    client.Client
	Log       logr.Logger
	Name      string
	DataStore kamajiv1alpha1.DataStore
}

func (r *ETCDCACertificatesResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Certificates.ETCD == nil {
		return true
	}

	return tenantControlPlane.Status.Certificates.ETCD.CA.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *ETCDCACertificatesResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *ETCDCACertificatesResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *ETCDCACertificatesResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *ETCDCACertificatesResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ETCDCACertificatesResource) GetName() string {
	return r.Name
}

func (r *ETCDCACertificatesResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Certificates.ETCD == nil {
		tenantControlPlane.Status.Certificates.ETCD = &kamajiv1alpha1.ETCDCertificatesStatus{}
	}

	tenantControlPlane.Status.Certificates.ETCD.CA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.ETCD.CA.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.ETCD.CA.Checksum = r.resource.GetAnnotations()["checksum"]

	return nil
}

func (r *ETCDCACertificatesResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *ETCDCACertificatesResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if r.DataStore.Spec.TLSConfig.CertificateAuthority.PrivateKey == nil {
			return fmt.Errorf("missing private key, cannot generate certificate for the given tenant control plane")
		}

		if etcdStatus := tenantControlPlane.Status.Certificates.ETCD; etcdStatus != nil && len(etcdStatus.CA.Checksum) > 0 && etcdStatus.CA.Checksum == r.resource.GetAnnotations()["checksum"] {
			isValid, err := etcd.IsETCDCertificateAndKeyPairValid(r.resource.Data[kubeadmconstants.CACertName], r.resource.Data[kubeadmconstants.CAKeyName])
			if err != nil {
				r.Log.Info(fmt.Sprintf("etcd certificates are not valid: %s", err.Error()))
			}

			if isValid {
				return nil
			}
		}

		ca, err := r.DataStore.Spec.TLSConfig.CertificateAuthority.Certificate.GetContent(ctx, r.Client)
		if err != nil {
			return err
		}

		key, err := r.DataStore.Spec.TLSConfig.CertificateAuthority.PrivateKey.GetContent(ctx, r.Client)
		if err != nil {
			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.CACertName: ca,
			kubeadmconstants.CAKeyName:  key,
		}

		r.resource.SetLabels(utilities.KamajiLabels())

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations["checksum"] = utilities.CalculateConfigMapChecksum(r.resource.StringData)
		r.resource.SetAnnotations(annotations)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
