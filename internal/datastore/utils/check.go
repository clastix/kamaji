// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// CheckExists ensures that the default Datastore exists before starting the manager.
func CheckExists(ctx context.Context, scheme *runtime.Scheme, datastoreName string) error {
	ctrlClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("unable to create controlerruntime.Client: %w", err)
	}

	if err = ctrlClient.Get(ctx, types.NamespacedName{Name: datastoreName}, &kamajiv1alpha1.DataStore{}); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("the default Datastore %s doesn't exist", datastoreName)
		}

		return fmt.Errorf("an error occurred during datastore retrieval: %w", err)
	}

	return nil
}
