// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type ReadOnly struct{}

func (r ReadOnly) GetHistogram() prometheus.Histogram {
	return prometheus.NewHistogram(prometheus.HistogramOpts{})
}

func (r ReadOnly) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (r ReadOnly) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r ReadOnly) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r ReadOnly) CreateOrUpdate(context.Context, *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.OperationResultNone, nil
}

func (r ReadOnly) GetName() string {
	return "readOnly"
}

func (r ReadOnly) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	if tcp.Status.Kubernetes.Version.Status == nil {
		return false
	}

	if *tcp.Status.Kubernetes.Version.Status == kamajiv1alpha1.VersionReadOnly && !tcp.Spec.ReadOnly {
		return true
	}

	if *tcp.Status.Kubernetes.Version.Status != kamajiv1alpha1.VersionReadOnly && tcp.Spec.ReadOnly {
		return true
	}

	return false
}

func (r ReadOnly) UpdateTenantControlPlaneStatus(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	if tcp.Spec.ReadOnly {
		tcp.Status.Kubernetes.Version.Status = ptr.To(kamajiv1alpha1.VersionReadOnly)

		return nil
	}

	tcp.Status.Kubernetes.Version.Status = ptr.To(kamajiv1alpha1.VersionReady)

	return nil
}
