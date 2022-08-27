// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateOrUpdateWithConflict is a helper function that wraps the RetryOnConflict around the CreateOrUpdate function:
// this allows to fetch from the cache the latest modified object an try to apply the changes defined in the MutateFn
// without enqueuing back the request in order to get the latest changes of the resource.
func CreateOrUpdateWithConflict(ctx context.Context, client client.Client, resource client.Object, f controllerutil.MutateFn) (res controllerutil.OperationResult, err error) {
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (scopeErr error) {
		if scopeErr = client.Get(ctx, k8stypes.NamespacedName{Namespace: resource.GetNamespace(), Name: resource.GetName()}, resource); scopeErr != nil {
			if !errors.IsNotFound(scopeErr) {
				return scopeErr
			}
		}

		res, scopeErr = controllerutil.CreateOrUpdate(ctx, client, resource, f)

		return scopeErr
	})

	return
}
