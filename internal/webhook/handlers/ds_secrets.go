// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type DataStoreSecretValidation struct {
	Client client.Client
}

func (d DataStoreSecretValidation) OnCreate(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (d DataStoreSecretValidation) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (d DataStoreSecretValidation) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		secret := object.(*corev1.Secret) //nolint:forcetypeassert

		dsList := &kamajiv1alpha1.DataStoreList{}

		if err := d.Client.List(ctx, dsList, client.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector(kamajiv1alpha1.DatastoreUsedSecretNamespacedNameKey, fmt.Sprintf("%s/%s", secret.GetNamespace(), secret.GetName()))}); err != nil {
			return nil, fmt.Errorf("cannot list Tenant Control Plane using the provided Secret: %w", err)
		}

		if len(dsList.Items) > 0 {
			var res []string

			for _, ds := range dsList.Items {
				res = append(res, ds.GetName())
			}

			return nil, fmt.Errorf("the Secret is used by the following kamajiv1alpha1.DataStores and cannot be deleted (%s)", strings.Join(res, ", "))
		}

		return nil, nil
	}
}
