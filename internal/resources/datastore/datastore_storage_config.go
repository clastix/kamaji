// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type Config struct {
	resource   *corev1.Secret
	Client     client.Client
	ConnString string
	DataStore  kamajiv1alpha1.DataStore
}

func (r *Config) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Storage.Config.Checksum != utilities.GetObjectChecksum(r.resource) ||
		tenantControlPlane.Status.Storage.DataStoreName != r.DataStore.GetName()
}

func (r *Config) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *Config) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *Config) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *Config) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *Config) GetClient() client.Client {
	return r.Client
}

func (r *Config) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *Config) GetName() string {
	return "datastore-config"
}

func (r *Config) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Storage.Driver = string(r.DataStore.Spec.Driver)
	tenantControlPlane.Status.Storage.DataStoreName = r.DataStore.GetName()
	tenantControlPlane.Status.Storage.Config.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Storage.Config.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

func (r *Config) mutate(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		var password []byte

		hash := utilities.GetObjectChecksum(r.resource)
		switch {
		case len(hash) > 0 && hash == utilities.CalculateMapChecksum(r.resource.Data):
			password = r.resource.Data["DB_PASSWORD"]
		default:
			password = []byte(uuid.New().String())
		}
		// the coalesce function prioritizes the return value stored in the TenantControlPlane status,
		// although this is going to be populated by the UpdateTenantControlPlaneStatus handler of the resource datastore-setup:
		// the default value will be used for fresh new configurations, and preserving a previous one:
		// this will keep us safe from naming changes cases as occurred with the following commit:
		// https://github.com/clastix/kamaji/pull/203/commits/09ce38f489cccca72ab728a259bc8fb2cf6e4770
		coalesceFn := func(fromStatus string) []byte {
			if len(fromStatus) > 0 {
				return []byte(fromStatus)
			}
			// The dash character (-) must be replaced with an underscore, PostgreSQL is complaining about it:
			// https://github.com/clastix/kamaji/issues/328
			return []byte(strings.ReplaceAll(fmt.Sprintf("%s_%s", tenantControlPlane.GetNamespace(), tenantControlPlane.GetName()), "-", "_"))
		}

		r.resource.Data = map[string][]byte{
			"DB_CONNECTION_STRING": []byte(r.ConnString),
			"DB_SCHEMA":            coalesceFn(tenantControlPlane.Status.Storage.Setup.Schema),
			"DB_USER":              coalesceFn(tenantControlPlane.Status.Storage.Setup.User),
			"DB_PASSWORD":          password,
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		r.resource.SetLabels(utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()))

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
