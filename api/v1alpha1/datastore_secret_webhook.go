// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:webhook:path=/validate--v1-secret,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",resources=secrets,verbs=delete,versions=v1,name=vdatastoresecrets.kb.io,admissionReviewVersions=v1

type dataStoreSecretValidator struct {
	log    logr.Logger
	client client.Client
}

func (d *dataStoreSecretValidator) ValidateCreate(context.Context, runtime.Object) error {
	return nil
}

func (d *dataStoreSecretValidator) ValidateUpdate(context.Context, runtime.Object, runtime.Object) error {
	return nil
}

func (d *dataStoreSecretValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	secret := obj.(*corev1.Secret) //nolint:forcetypeassert

	dsList := &DataStoreList{}

	if err := d.client.List(ctx, dsList, client.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector(DatastoreUsedSecretNamespacedNameKey, fmt.Sprintf("%s/%s", secret.GetNamespace(), secret.GetName()))}); err != nil {
		return err
	}

	if len(dsList.Items) > 0 {
		var res []string

		for _, ds := range dsList.Items {
			res = append(res, ds.GetName())
		}

		return fmt.Errorf("the Secret is used by the following kamajiv1alpha1.DataStores and cannot be deleted (%s)", strings.Join(res, ", "))
	}

	return nil
}

func (d *dataStoreSecretValidator) Default(context.Context, runtime.Object) error {
	return nil
}
