// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/types"
	"github.com/clastix/kamaji/internal/utilities"
)

type SQLCertificate struct {
	resource    *corev1.Secret
	Client      client.Client
	Name        string
	StorageType types.ETCDStorageType
	DataStore   kamajiv1alpha1.DataStore
}

func (r *SQLCertificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Storage.Kine.Certificate.Checksum != r.resource.GetAnnotations()["checksum"]
}

func (r *SQLCertificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *SQLCertificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *SQLCertificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
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
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *SQLCertificate) GetClient() client.Client {
	return r.Client
}

func (r *SQLCertificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *SQLCertificate) GetName() string {
	return r.Name
}

func (r *SQLCertificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Storage.Kine == nil {
		tenantControlPlane.Status.Storage.Kine = &kamajiv1alpha1.KineStatus{}
	}

	tenantControlPlane.Status.Storage.Kine.Certificate.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Storage.Kine.Certificate.Checksum = r.resource.GetAnnotations()["checksum"]
	tenantControlPlane.Status.Storage.Kine.Certificate.LastUpdate = metav1.Now()

	return nil
}

func (r *SQLCertificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		ca, err := r.DataStore.Spec.TLSConfig.CertificateAuthority.Certificate.GetContent(ctx, r.Client)
		if err != nil {
			return nil
		}

		crt, err := r.DataStore.Spec.TLSConfig.ClientCertificate.Certificate.GetContent(ctx, r.Client)
		if err != nil {
			return nil
		}

		key, err := r.DataStore.Spec.TLSConfig.ClientCertificate.PrivateKey.GetContent(ctx, r.Client)
		if err != nil {
			return nil
		}

		r.resource.Data = map[string][]byte{
			"ca.crt":     ca,
			"server.crt": crt,
			"server.key": key,
		}

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations["checksum"] = utilities.CalculateConfigMapChecksum(r.resource.StringData)

		r.resource.SetAnnotations(annotations)

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
