// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func listTenantControlPlanesUsingDataStore(ctx context.Context, c client.Client, dataStoreName string) ([]kamajiv1alpha1.TenantControlPlane, error) {
	var tcpList kamajiv1alpha1.TenantControlPlaneList

	if err := c.List(ctx, &tcpList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, dataStoreName),
	}); err != nil {
		return nil, err
	}

	return tcpList.Items, nil
}
