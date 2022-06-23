// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/types"
	"github.com/clastix/kamaji/internal/utilities"
)

type SQLCertificate struct {
	resource                 *corev1.Secret
	Client                   client.Client
	Name                     string
	StorageType              types.ETCDStorageType
	SQLConfigSecretName      string
	SQLConfigSecretNamespace string
}

func (r *SQLCertificate) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Storage.KineMySQL.Certificate.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Storage.KineMySQL.Certificate.ResourceVersion != r.resource.ResourceVersion
}

func (r *SQLCertificate) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *SQLCertificate) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *SQLCertificate) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
		Data: map[string][]byte{},
	}

	return nil
}

func (r *SQLCertificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *SQLCertificate) GetClient() client.Client {
	return r.Client
}

func (r *SQLCertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *SQLCertificate) GetName() string {
	return r.Name
}

func (r *SQLCertificate) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Storage.KineMySQL == nil {
		tenantControlPlane.Status.Storage.KineMySQL = &kamajiv1alpha1.KineMySQLStatus{}
	}

	tenantControlPlane.Status.Storage.KineMySQL.Certificate.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Storage.KineMySQL.Certificate.ResourceVersion = r.resource.ResourceVersion
	tenantControlPlane.Status.Storage.KineMySQL.Certificate.LastUpdate = metav1.Now()

	return nil
}

func (r *SQLCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		sqlConfig := &corev1.Secret{}
		namespacedName := k8stypes.NamespacedName{Namespace: r.SQLConfigSecretNamespace, Name: r.SQLConfigSecretName}
		if err := r.Client.Get(ctx, namespacedName, sqlConfig); err != nil {
			return err
		}

		if err := r.buildSecret(ctx, *sqlConfig); err != nil {
			return err
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			r.resource.GetLabels(),
			map[string]string{
				"kamaji.clastix.io/name":      tenantControlPlane.GetName(),
				"kamaji.clastix.io/component": r.GetName(),
			},
		))

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *SQLCertificate) buildSecret(ctx context.Context, sqlConfig corev1.Secret) error {
	switch r.StorageType {
	case types.KineMySQL:
		keys := []string{"ca.crt", "server.crt", "server.key"}

		return r.buildKineSecret(ctx, keys, sqlConfig)
	default:
		return fmt.Errorf("storage type %s is not implemented", r.StorageType)
	}
}

func (r *SQLCertificate) buildKineSecret(ctx context.Context, keys []string, sqlConfig corev1.Secret) error {
	for _, key := range keys {
		value, ok := sqlConfig.Data[key]
		if !ok {
			return fmt.Errorf("%s is not in sql config secret", key)
		}

		r.resource.Data[key] = value
	}

	return nil
}
