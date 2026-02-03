// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

type MultiTenancy struct {
	DataStore kamajiv1alpha1.DataStore
}

func (m *MultiTenancy) GetHistogram() prometheus.Histogram {
	multiTenancyCollector = resources.LazyLoadHistogramFromResource(multiTenancyCollector, m)

	return multiTenancyCollector
}

func (m *MultiTenancy) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (m *MultiTenancy) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (m *MultiTenancy) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (m *MultiTenancy) CreateOrUpdate(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	// If the NATS Datastore is already used by a Tenant Control Plane
	// and a new one is reclaiming it, we need to stop it, since it's not allowed.
	// TODO(prometherion): remove this after multi-tenancy is implemented for NATS.
	if m.DataStore.Spec.Driver != kamajiv1alpha1.KineNatsDriver {
		return controllerutil.OperationResultNone, nil
	}

	usedBy := sets.New[string](m.DataStore.Status.UsedBy...)

	switch {
	case usedBy.Has(tcp.Namespace + "/" + tcp.Name):
		return controllerutil.OperationResultNone, nil
	case usedBy.Len() == 0:
		return controllerutil.OperationResultNone, nil
	default:
		return controllerutil.OperationResultNone, errors.New("NATS doesn't support multi-tenancy, the current datastore is already in use")
	}
}

func (m *MultiTenancy) GetName() string {
	return "ds.multitenancy"
}

func (m *MultiTenancy) ShouldStatusBeUpdated(context.Context, *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (m *MultiTenancy) UpdateTenantControlPlaneStatus(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}
