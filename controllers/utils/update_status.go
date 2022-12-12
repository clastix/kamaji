// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/internal/resources"
)

func UpdateStatus(ctx context.Context, client client.Client, tcpRetrieval TenantControlPlaneRetrievalFn, resource resources.Resource) error {
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		tenantControlPlane, err := tcpRetrieval()
		if err != nil {
			return err
		}

		if err = resource.UpdateTenantControlPlaneStatus(ctx, tenantControlPlane); err != nil {
			return fmt.Errorf("error applying TenantcontrolPlane status: %w", err)
		}

		if err = client.Status().Update(ctx, tenantControlPlane); err != nil {
			return fmt.Errorf("error updating tenantControlPlane status: %w", err)
		}

		return nil
	})

	return updateErr
}
