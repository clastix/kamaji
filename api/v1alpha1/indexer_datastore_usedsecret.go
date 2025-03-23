// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DatastoreUsedSecretNamespacedNameKey = "secretRef"
)

type DatastoreUsedSecret struct{}

func (d *DatastoreUsedSecret) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, d.Object(), d.Field(), d.ExtractValue())
}

func (d *DatastoreUsedSecret) Object() client.Object {
	return &DataStore{}
}

func (d *DatastoreUsedSecret) Field() string {
	return DatastoreUsedSecretNamespacedNameKey
}

func (d *DatastoreUsedSecret) ExtractValue() client.IndexerFunc {
	return func(object client.Object) (res []string) {
		ds := object.(*DataStore) //nolint:forcetypeassert

		if ds.Spec.BasicAuth != nil {
			if ds.Spec.BasicAuth.Username.SecretRef != nil {
				res = append(res, d.namespacedName(*ds.Spec.BasicAuth.Username.SecretRef))
			}

			if ds.Spec.BasicAuth.Password.SecretRef != nil {
				res = append(res, d.namespacedName(*ds.Spec.BasicAuth.Password.SecretRef))
			}
		}

		if ds.Spec.TLSConfig != nil {
			if ds.Spec.TLSConfig.CertificateAuthority.Certificate.SecretRef != nil {
				res = append(res, d.namespacedName(*ds.Spec.TLSConfig.CertificateAuthority.Certificate.SecretRef))
			}

			if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey != nil && ds.Spec.TLSConfig.CertificateAuthority.PrivateKey.SecretRef != nil {
				res = append(res, d.namespacedName(*ds.Spec.TLSConfig.CertificateAuthority.PrivateKey.SecretRef))
			}

			if ds.Spec.TLSConfig.ClientCertificate != nil {
				if ds.Spec.TLSConfig.ClientCertificate.Certificate.SecretRef != nil {
					res = append(res, d.namespacedName(*ds.Spec.TLSConfig.ClientCertificate.Certificate.SecretRef))
				}

				if ds.Spec.TLSConfig.ClientCertificate.PrivateKey.SecretRef != nil {
					res = append(res, d.namespacedName(*ds.Spec.TLSConfig.ClientCertificate.PrivateKey.SecretRef))
				}
			}
		}

		return res
	}
}

func (d *DatastoreUsedSecret) namespacedName(ref SecretReference) string {
	return fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
}
