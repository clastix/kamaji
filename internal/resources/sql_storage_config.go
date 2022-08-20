// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type SQLStorageConfig struct {
	resource *corev1.Secret
	Client   client.Client
	Name     string
	Host     string
	Port     int
	Driver   string
}

func (r *SQLStorageConfig) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Storage.Kine == nil {
		return true
	}

	return tenantControlPlane.Status.Storage.Kine.Config.SecretName != r.resource.GetName() ||
		tenantControlPlane.Status.Storage.Kine.Config.Checksum != r.resource.GetAnnotations()["checksum"] ||
		tenantControlPlane.Status.Storage.Kine.Driver != r.Driver
}

func (r *SQLStorageConfig) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *SQLStorageConfig) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *SQLStorageConfig) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *SQLStorageConfig) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}

func (r *SQLStorageConfig) GetClient() client.Client {
	return r.Client
}

func (r *SQLStorageConfig) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *SQLStorageConfig) GetName() string {
	return r.Name
}

func (r *SQLStorageConfig) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Storage.Kine == nil {
		tenantControlPlane.Status.Storage.Kine = &kamajiv1alpha1.KineStatus{}
	}

	tenantControlPlane.Status.Storage.Kine.Driver = r.Driver
	tenantControlPlane.Status.Storage.Kine.Config.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Storage.Kine.Config.Checksum = r.resource.GetAnnotations()["checksum"]

	return nil
}

func (r *SQLStorageConfig) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		var password []byte

		savedHash, ok := r.resource.GetAnnotations()["checksum"]
		switch {
		case ok && savedHash == utilities.CalculateConfigMapChecksum(r.resource.StringData):
			password = r.resource.Data["DB_PASSWORD"]
		default:
			password = []byte(utilities.GenerateUUIDString())
		}

		r.resource.Data = map[string][]byte{
			"DB_HOST":     []byte(r.Host),
			"DB_PORT":     []byte(strconv.Itoa(r.Port)),
			"DB_SCHEMA":   []byte(tenantControlPlane.GetName()),
			"DB_USER":     []byte(tenantControlPlane.GetName()),
			"DB_PASSWORD": password,
		}

		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations["checksum"] = utilities.CalculateConfigMapChecksum(r.resource.StringData)
		r.resource.SetAnnotations(annotations)

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
