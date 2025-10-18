// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type Certificate struct {
	resource                *corev1.Secret
	Client                  client.Client
	Name                    string
	DataStore               kamajiv1alpha1.DataStore
	CertExpirationThreshold time.Duration
}

func (r *Certificate) GetHistogram() prometheus.Histogram {
	certificateCollector = resources.LazyLoadHistogramFromResource(certificateCollector, r)

	return certificateCollector
}

func (r *Certificate) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Storage.Certificate.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *Certificate) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *Certificate) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *Certificate) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *Certificate) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *Certificate) GetClient() client.Client {
	return r.Client
}

func (r *Certificate) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *Certificate) GetName() string {
	return "datastore-certificate"
}

func (r *Certificate) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Storage.Certificate.SecretName = r.resource.GetName()
	tenantControlPlane.Status.Storage.Certificate.Checksum = utilities.GetObjectChecksum(r.resource)
	tenantControlPlane.Status.Storage.Certificate.LastUpdate = metav1.Now()

	return nil
}

func (r *Certificate) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		isRotationRequested := utilities.IsRotationRequested(r.resource)

		if r.DataStore.Spec.TLSConfig != nil {
			ca, err := r.DataStore.Spec.TLSConfig.CertificateAuthority.Certificate.GetContent(ctx, r.Client)
			if err != nil {
				logger.Error(err, "cannot retrieve CA certificate content")

				return err
			}

			if r.resource.Data == nil {
				r.resource.Data = map[string][]byte{}
			}

			r.resource.Data["ca.crt"] = ca

			r.resource.SetLabels(utilities.MergeMaps(
				r.resource.GetLabels(),
				utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()),
				map[string]string{
					constants.ControllerLabelResource: utilities.CertificateX509Label,
				},
			))

			if err = ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme()); err != nil {
				logger.Error(err, "cannot set controller reference", "resource", r.GetName())

				return err
			}

			if utilities.GetObjectChecksum(r.resource) == utilities.CalculateMapChecksum(r.resource.Data) {
				if r.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
					if isValid, _ := crypto.IsValidCertificateKeyPairBytes(r.resource.Data["server.crt"], r.resource.Data["server.key"], r.CertExpirationThreshold); isValid && !isRotationRequested {
						return nil
					}
				}
			}

			var crt, key *bytes.Buffer

			switch r.DataStore.Spec.Driver {
			case kamajiv1alpha1.EtcdDriver:
				var privateKey []byte
				// When dealing with the etcd storage we cannot use the basic authentication, thus the generation of a
				// certificate used for authentication is mandatory, along with the CA private key.
				if privateKey, err = r.DataStore.Spec.TLSConfig.CertificateAuthority.PrivateKey.GetContent(ctx, r.Client); err != nil {
					logger.Error(err, "unable to retrieve CA private key content")

					return err
				}

				if crt, key, err = crypto.GenerateCertificatePrivateKeyPair(crypto.NewCertificateTemplate(tenantControlPlane.Status.Storage.Setup.User), ca, privateKey); err != nil {
					logger.Error(err, "unable to generate certificate and private key")

					return err
				}
			case kamajiv1alpha1.KineMySQLDriver, kamajiv1alpha1.KinePostgreSQLDriver, kamajiv1alpha1.KineNatsDriver:
				var crtBytes, keyBytes []byte
				// For the SQL drivers we just need to copy the certificate, since the basic authentication is used
				// to connect to the desired schema and database.

				if r.DataStore.Spec.TLSConfig.ClientCertificate != nil {
					if crtBytes, err = r.DataStore.Spec.TLSConfig.ClientCertificate.Certificate.GetContent(ctx, r.Client); err != nil {
						logger.Error(err, "unable to retrieve certificate content")

						return err
					}

					crt = bytes.NewBuffer(crtBytes)

					if keyBytes, err = r.DataStore.Spec.TLSConfig.ClientCertificate.PrivateKey.GetContent(ctx, r.Client); err != nil {
						logger.Error(err, "unable to retrieve private key content")

						return err
					}
					key = bytes.NewBuffer(keyBytes)
				}
			default:
				return fmt.Errorf("unrecognized driver for Certificate generation")
			}

			if r.DataStore.Spec.TLSConfig.ClientCertificate != nil {
				r.resource.Data["server.crt"] = crt.Bytes()
				r.resource.Data["server.key"] = key.Bytes()
			}
		} else {
			// set r.resource.Data to empty to allow switching from TLS to non-tls
			r.resource.Data = map[string][]byte{}
		}

		if isRotationRequested {
			utilities.SetLastRotationTimestamp(r.resource)
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return nil
	}
}
