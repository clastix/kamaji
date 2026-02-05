// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type Config struct {
	resource   *corev1.Secret
	Client     client.Client
	ConnString string
	DataStore  kamajiv1alpha1.DataStore
	IsOverride bool
}

func (r *Config) GetHistogram() prometheus.Histogram {
	storageCollector = resources.LazyLoadHistogramFromResource(storageCollector, r)

	return storageCollector
}

func (r *Config) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Storage.Config.Checksum != utilities.GetObjectChecksum(r.resource) ||
		tenantControlPlane.Status.Storage.DataStoreName != r.DataStore.GetName()
}

func (r *Config) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *Config) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *Config) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utilities.AddTenantPrefix(r.GetName(), tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *Config) GetClient() client.Client {
	return r.Client
}

func (r *Config) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

// Delete doesn't perform any deletion process: the Secret object has owner relationship
// with the TenantControlPlane object, which has been previously deleted.
func (r *Config) Delete(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) error {
	secret := r.resource.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: r.resource.Name, Namespace: r.resource.Namespace}, secret); err != nil {
			if kubeerrors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("cannot retrieve the DataStore Secret for removal: %w", err)
		}

		secret.SetFinalizers(nil)

		if err := r.Client.Update(ctx, secret); err != nil {
			if kubeerrors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("cannot remove DataStore Secret finalizers: %w", err)
		}

		return nil
	})
}

func (r *Config) GetName() string {
	return "datastore-config"
}

func (r *Config) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if !r.IsOverride {
		tenantControlPlane.Status.Storage.Driver = string(r.DataStore.Spec.Driver)
		tenantControlPlane.Status.Storage.DataStoreName = r.DataStore.GetName()
		tenantControlPlane.Status.Storage.Config.SecretName = r.resource.GetName()
		tenantControlPlane.Status.Storage.Config.Checksum = utilities.GetObjectChecksum(r.resource)
	}

	return nil
}

func (r *Config) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		var password []byte
		var username []byte

		hash := utilities.GetObjectChecksum(r.resource)
		switch {
		case len(hash) > 0 && hash == utilities.CalculateMapChecksum(r.resource.Data):
			password = r.resource.Data["DB_PASSWORD"]
		default:
			password = []byte(uuid.New().String())
		}

		finalizersList := sets.New[string](r.resource.GetFinalizers()...)
		finalizersList.Insert(finalizers.DatastoreSecretFinalizer)
		r.resource.SetFinalizers(finalizersList.UnsortedList())

		// TODO(thecodeassassin): remove this after multi-tenancy is implemented for NATS.
		// Due to NATS is missing a programmatic approach to create users and password,
		// we're using the Datastore root password.
		if r.DataStore.Spec.Driver == kamajiv1alpha1.KineNatsDriver {
			// set username and password to the basicAuth values of the NATS datastore
			u, err := r.DataStore.Spec.BasicAuth.Username.GetContent(ctx, r.Client)
			if err != nil {
				return fmt.Errorf("failed to retrieve the username for the NATS datastore: %w", err)
			}

			p, err := r.DataStore.Spec.BasicAuth.Password.GetContent(ctx, r.Client)
			if err != nil {
				return fmt.Errorf("failed to retrieve the password for the NATS datastore: %w", err)
			}

			username = u
			password = p
		} else {
			// prioritize the username stored in the TenantControlPlane status,
			// although this is going to be populated by the UpdateTenantControlPlaneStatus handler of the resource datastore-setup:
			// the default value will be used for fresh new configurations, and preserving a previous one:
			// this will keep us safe from naming changes cases as occurred with the following commit:
			// https://github.com/clastix/kamaji/pull/203/commits/09ce38f489cccca72ab728a259bc8fb2cf6e4770
			switch {
			case len(tenantControlPlane.Status.Storage.Setup.User) > 0:
				// for existing TCPs, the dataStoreSchema will be adopted from the status,
				// as the mutating webhook only takes care of TCP creations, not updates
				username = []byte(tenantControlPlane.Status.Storage.Setup.User)
				tenantControlPlane.Spec.DataStoreUsername = string(username)
			case len(tenantControlPlane.Spec.DataStoreUsername) > 0:
				// for new TCPs, the spec field will have been provided by the user
				// or defaulted by the defaulting webhook
				username = []byte(tenantControlPlane.Spec.DataStoreUsername)
			default:
				username = []byte(tenantControlPlane.GetDefaultDatastoreUsername())
			}
		}

		var dataStoreSchema string
		switch {
		case len(tenantControlPlane.Status.Storage.Setup.Schema) > 0:
			// for existing TCPs, the dataStoreSchema will be adopted from the status,
			// as the mutating webhook only takes care of TCP creations, not updates
			dataStoreSchema = tenantControlPlane.Status.Storage.Setup.Schema
			tenantControlPlane.Spec.DataStoreSchema = dataStoreSchema
		case len(tenantControlPlane.Spec.DataStoreSchema) > 0:
			// for new TCPs, the spec field will have been provided by the user
			// or defaulted by the defaulting webhook
			dataStoreSchema = tenantControlPlane.Spec.DataStoreSchema
		default:
			dataStoreSchema = tenantControlPlane.GetDefaultDatastoreSchema()
		}

		r.resource.Data = map[string][]byte{
			"DB_CONNECTION_STRING": []byte(r.ConnString),
			"DB_SCHEMA":            []byte(dataStoreSchema),
			"DB_USER":              username,
			"DB_PASSWORD":          password,
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName())))

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
