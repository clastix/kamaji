// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package indexers

import (
	"context"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

const (
	TenantControlPlaneUsedDataStoreKey = "status.storage.dataStoreName"
)

type TenantControlPlaneStatusDataStore struct{}

func (t *TenantControlPlaneStatusDataStore) Object() client.Object {
	return &kamajiv1alpha1.TenantControlPlane{}
}

func (t *TenantControlPlaneStatusDataStore) Field() string {
	return TenantControlPlaneUsedDataStoreKey
}

func (t *TenantControlPlaneStatusDataStore) ExtractValue() client.IndexerFunc {
	return func(object client.Object) []string {
		//nolint:forcetypeassert
		tcp := object.(*kamajiv1alpha1.TenantControlPlane)

		return []string{tcp.Status.Storage.DataStoreName}
	}
}

func (t *TenantControlPlaneStatusDataStore) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, t.Object(), t.Field(), t.ExtractValue())
}
