// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TenantControlPlaneUsedDataStoreKey = "status.storage.dataStoreName"
)

type TenantControlPlaneStatusDataStore struct{}

func (t *TenantControlPlaneStatusDataStore) Object() client.Object {
	return &TenantControlPlane{}
}

func (t *TenantControlPlaneStatusDataStore) Field() string {
	return TenantControlPlaneUsedDataStoreKey
}

func (t *TenantControlPlaneStatusDataStore) ExtractValue() client.IndexerFunc {
	return func(object client.Object) []string {
		tcp := object.(*TenantControlPlane) //nolint:forcetypeassert

		return []string{tcp.Status.Storage.DataStoreName}
	}
}

func (t *TenantControlPlaneStatusDataStore) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, t.Object(), t.Field(), t.ExtractValue())
}
