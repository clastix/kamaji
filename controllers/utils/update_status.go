// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

func UpdateStatus(ctx context.Context, client client.Client, tcp *kamajiv1alpha1.TenantControlPlane, resource resources.Resource) error {
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		defer func() {
			if err != nil {
				_ = client.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)
			}
		}()

		if err = resource.UpdateTenantControlPlaneStatus(ctx, tcp); err != nil {
			return fmt.Errorf("error applying TenantcontrolPlane status: %w", err)
		}

		tcp.Status.ObservedGeneration = tcp.Generation

		if err = client.Status().Update(ctx, tcp); err != nil {
			return fmt.Errorf("error updating tenantControlPlane status: %w", err)
		}

		return nil
	})

	return updateErr
}
