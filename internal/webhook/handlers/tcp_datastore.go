// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneDataStore struct {
	Client client.Client
}

func (t TenantControlPlaneDataStore) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if tcp.Spec.DataStore != "" {
			return nil, t.check(ctx, tcp.Spec.DataStore)
		}

		return nil, t.checkDataStoreOverrides(ctx, tcp)
	}
}

func (t TenantControlPlaneDataStore) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneDataStore) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if tcp.Spec.DataStore != "" {
			return nil, t.check(ctx, tcp.Spec.DataStore)
		}

		return nil, nil
	}
}

func (t TenantControlPlaneDataStore) check(ctx context.Context, dataStoreName string) error {
	if err := t.Client.Get(ctx, types.NamespacedName{Name: dataStoreName}, &kamajiv1alpha1.DataStore{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("%s DataStore does not exist", dataStoreName)
		}

		return fmt.Errorf("an unexpected error occurred upon Tenant Control Plane DataStore check, %w", err)
	}

	return nil
}

func (t TenantControlPlaneDataStore) checkDataStoreOverrides(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	overrideCheck := make(map[string]struct{}, 0)
	for _, ds := range tcp.Spec.DataStoreOverrides {
		if _, exists := overrideCheck[ds.Resource]; !exists {
			overrideCheck[ds.Resource] = struct{}{}
		} else {
			return fmt.Errorf("duplicate resource override in Spec.DataStoreOverrides")
		}
		if err := t.check(ctx, ds.DataStore); err != nil {
			return err
		}
	}

	return nil
}
