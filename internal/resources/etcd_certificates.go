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
	"github.com/clastix/kamaji/internal/etcd"
	"github.com/clastix/kamaji/internal/utilities"
)

type ETCDCertificatesResource struct {
	resource *corev1.Secret
	Client   client.Client
	Log      logr.Logger
	Name     string
}

func (r *ETCDCertificatesResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Certificates.ETCD == nil {
		return true
	}

	return tenantControlPlane.Status.Certificates.ETCD.APIServer.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *ETCDCertificatesResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *ETCDCertificatesResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *ETCDCertificatesResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *ETCDCertificatesResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ETCDCertificatesResource) GetName() string {
	return r.Name
}

func (r *ETCDCertificatesResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Certificates.ETCD == nil {
		tenantControlPlane.Status.Certificates.ETCD = &kamajiv1alpha1.ETCDCertificatesStatus{}
	}

	tenantControlPlane.Status.Certificates.ETCD.APIServer.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.ETCD.APIServer.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Certificates.ETCD.APIServer.Checksum = r.resource.GetAnnotations()["checksum"]

	return nil
}

func (r *ETCDCertificatesResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *ETCDCertificatesResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if tenantControlPlane.Status.Certificates.ETCD == nil {
			return fmt.Errorf("etcd is still synchronizing latest changes")
		}

		if checksum := tenantControlPlane.Status.Certificates.ETCD.APIServer.Checksum; len(checksum) > 0 && checksum == r.resource.GetAnnotations()["checksum"] {
			isValid, err := etcd.IsETCDCertificateAndKeyPairValid(r.resource.Data[kubeadmconstants.APIServerEtcdClientCertName], r.resource.Data[kubeadmconstants.APIServerEtcdClientKeyName])
			if err != nil {
				r.Log.Info(fmt.Sprintf("etcd certificates are not valid: %s", err.Error()))
			}

			if isValid {
				return nil
			}
		}

		etcdCASecretNamespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.ETCD.CA.SecretName}
		etcdCASecret := &corev1.Secret{}
		if err := r.Client.Get(ctx, etcdCASecretNamespacedName, etcdCASecret); err != nil {
			return err
		}

		cert, privKey, err := etcd.GetETCDCACertificateAndKeyPair(
			tenantControlPlane.GetName(),
			etcdCASecret.Data[kubeadmconstants.CACertName],
			etcdCASecret.Data[kubeadmconstants.CAKeyName],
		)
		if err != nil {
			return err
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.APIServerEtcdClientCertName: cert.Bytes(),
			kubeadmconstants.APIServerEtcdClientKeyName:  privKey.Bytes(),
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
