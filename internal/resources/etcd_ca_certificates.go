// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"reflect"

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

type ETCDCACertificatesResource struct {
	resource              *corev1.Secret
	Client                client.Client
	Log                   logr.Logger
	Name                  string
	ETCDCASecretName      string
	ETCDCASecretNamespace string
}

func (r *ETCDCACertificatesResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Certificates.ETCD == nil {
		return true
	}

	return tenantControlPlane.Status.Certificates.ETCD.CA.SecretName != r.resource.GetName()
}

func (r *ETCDCACertificatesResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *ETCDCACertificatesResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *ETCDCACertificatesResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *ETCDCACertificatesResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ETCDCACertificatesResource) GetName() string {
	return r.Name
}

func (r *ETCDCACertificatesResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Certificates.ETCD == nil {
		tenantControlPlane.Status.Certificates.ETCD = &kamajiv1alpha1.ETCDCertificatesStatus{}
	}

	tenantControlPlane.Status.Certificates.ETCD.CA.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Certificates.ETCD.CA.LastUpdate = metav1.Now()

	return nil
}

func (r *ETCDCACertificatesResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *ETCDCACertificatesResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.KamajiLabels())

		etcdCASecretNamespacedName := k8stypes.NamespacedName{Namespace: r.ETCDCASecretNamespace, Name: r.ETCDCASecretName}
		etcdCASecret := &corev1.Secret{}
		if err := r.Client.Get(ctx, etcdCASecretNamespacedName, etcdCASecret); err != nil {
			return err
		}

		isValid, err := etcd.IsETCDCertificateAndKeyPairValid(r.resource.Data[kubeadmconstants.CACertName], r.resource.Data[kubeadmconstants.CAKeyName])
		if err != nil {
			r.Log.Info(fmt.Sprintf("etcd certificates are not valid: %s", err.Error()))
		}

		if reflect.DeepEqual(etcdCASecret.Data[kubeadmconstants.CACertName], r.resource.Data[kubeadmconstants.CACertName]) &&
			reflect.DeepEqual(etcdCASecret.Data[kubeadmconstants.CAKeyName], r.resource.Data[kubeadmconstants.CAKeyName]) {
			if isValid {
				return nil
			}

			return fmt.Errorf("CA certificates provided into secrets %s/%s are not valid", r.ETCDCASecretNamespace, r.ETCDCASecretName)
		}

		r.resource.Data = map[string][]byte{
			kubeadmconstants.CACertName: etcdCASecret.Data[kubeadmconstants.CACertName],
			kubeadmconstants.CAKeyName:  etcdCASecret.Data[kubeadmconstants.CAKeyName],
		}

		if err = ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme()); err != nil {
			return err
		}

		return nil
	}
}
