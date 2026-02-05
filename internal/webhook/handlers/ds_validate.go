// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type DataStoreValidation struct {
	Client client.Client
}

func (d DataStoreValidation) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		ds := object.(*kamajiv1alpha1.DataStore) //nolint:forcetypeassert

		return nil, d.validate(ctx, *ds)
	}
}

func (d DataStoreValidation) OnDelete(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		ds := object.(*kamajiv1alpha1.DataStore) //nolint:forcetypeassert

		tcpList := &kamajiv1alpha1.TenantControlPlaneList{}
		if err := d.Client.List(ctx, tcpList, client.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector(kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, ds.GetName())}); err != nil {
			return nil, fmt.Errorf("cannot retrieve TenantControlPlane list used by the DataStore: %w", err)
		}

		if len(tcpList.Items) > 0 {
			return nil, fmt.Errorf("the DataStore is used by multiple TenantControlPlanes and cannot be removed")
		}

		return nil, nil
	}
}

func (d DataStoreValidation) OnUpdate(object runtime.Object, oldObj runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		newDs, oldDs := object.(*kamajiv1alpha1.DataStore), oldObj.(*kamajiv1alpha1.DataStore) //nolint:forcetypeassert

		if oldDs.Spec.Driver != newDs.Spec.Driver {
			return nil, fmt.Errorf("driver of a DataStore cannot be changed")
		}

		return nil, d.validate(ctx, *newDs)
	}
}

func (d DataStoreValidation) validate(ctx context.Context, ds kamajiv1alpha1.DataStore) error {
	if ds.Spec.BasicAuth != nil {
		if err := d.validateBasicAuth(ctx, ds); err != nil {
			return err
		}
	}

	return d.validateTLSConfig(ctx, ds)
}

func (d DataStoreValidation) validateBasicAuth(ctx context.Context, ds kamajiv1alpha1.DataStore) error {
	if err := d.validateContentReference(ctx, ds.Spec.BasicAuth.Password); err != nil {
		return fmt.Errorf("basic-auth password is not valid, %w", err)
	}

	if err := d.validateContentReference(ctx, ds.Spec.BasicAuth.Username); err != nil {
		return fmt.Errorf("basic-auth username is not valid, %w", err)
	}

	return nil
}

func (d DataStoreValidation) validateTLSConfig(ctx context.Context, ds kamajiv1alpha1.DataStore) error {
	if ds.Spec.TLSConfig == nil && ds.Spec.Driver != kamajiv1alpha1.EtcdDriver {
		return nil
	}

	if err := d.validateContentReference(ctx, ds.Spec.TLSConfig.CertificateAuthority.Certificate); err != nil {
		return fmt.Errorf("CA certificate is not valid, %w", err)
	}

	if ds.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey == nil {
			return fmt.Errorf("CA private key is required when using the etcd driver")
		}

		if ds.Spec.TLSConfig.ClientCertificate == nil {
			return fmt.Errorf("client certificate is required when using the etcd driver")
		}
	}

	if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey != nil {
		if err := d.validateContentReference(ctx, *ds.Spec.TLSConfig.CertificateAuthority.PrivateKey); err != nil {
			return fmt.Errorf("CA private key is not valid, %w", err)
		}
	}

	if ds.Spec.TLSConfig.ClientCertificate != nil {
		if err := d.validateContentReference(ctx, ds.Spec.TLSConfig.ClientCertificate.Certificate); err != nil {
			return fmt.Errorf("client certificate is not valid, %w", err)
		}

		if err := d.validateContentReference(ctx, ds.Spec.TLSConfig.ClientCertificate.PrivateKey); err != nil {
			return fmt.Errorf("client private key is not valid, %w", err)
		}
	}

	return nil
}

func (d DataStoreValidation) validateContentReference(ctx context.Context, ref kamajiv1alpha1.ContentRef) error {
	switch {
	case len(ref.Content) > 0:
		return nil
	case ref.SecretRef == nil:
		return fmt.Errorf("the Secret reference is mandatory when bare content is not specified")
	case len(ref.SecretRef.SecretReference.Name) == 0:
		return fmt.Errorf("the Secret reference name is mandatory")
	case len(ref.SecretRef.SecretReference.Namespace) == 0:
		return fmt.Errorf("the Secret reference namespace is mandatory")
	}

	if err := d.Client.Get(ctx, types.NamespacedName{Name: ref.SecretRef.SecretReference.Name, Namespace: ref.SecretRef.SecretReference.Namespace}, &corev1.Secret{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("secret %s/%s is not found", ref.SecretRef.SecretReference.Namespace, ref.SecretRef.SecretReference.Name)
		}

		return err
	}

	return nil
}
