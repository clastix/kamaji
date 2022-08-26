// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/datastore"
	"github.com/clastix/kamaji/internal/resources/utils"
)

type SetupResource struct {
	schema   string
	user     string
	password string
}

type Setup struct {
	resource   SetupResource
	Client     client.Client
	Connection datastore.Connection
	DataStore  kamajiv1alpha1.DataStore
}

func (r *Setup) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Storage.Driver != string(r.DataStore.Spec.Driver) &&
		tenantControlPlane.Status.Storage.Setup.Checksum != tenantControlPlane.Status.Storage.Config.Checksum
}

func (r *Setup) ShouldCleanup(_ *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *Setup) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *Setup) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: tenantControlPlane.GetNamespace(),
		Name:      tenantControlPlane.Status.Storage.Config.SecretName,
	}
	if err := r.Client.Get(ctx, namespacedName, secret); err != nil {
		return err
	}

	r.resource = SetupResource{
		schema:   string(secret.Data["DB_SCHEMA"]),
		user:     string(secret.Data["DB_USER"]),
		password: string(secret.Data["DB_PASSWORD"]),
	}

	return nil
}

func (r *Setup) GetClient() client.Client {
	return r.Client
}

func (r *Setup) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tenantControlPlane.Status.Storage.Setup.Checksum != "" &&
		tenantControlPlane.Status.Storage.Setup.Checksum != tenantControlPlane.Status.Storage.Config.Checksum {
		if err := r.Delete(ctx, tenantControlPlane); err != nil {
			return controllerutil.OperationResultNone, err
		}

		return controllerutil.OperationResultUpdated, nil
	}

	reconcilationResult := controllerutil.OperationResultNone
	var operationResult controllerutil.OperationResult
	var err error

	operationResult, err = r.createDB(ctx, tenantControlPlane)
	if err != nil {
		return reconcilationResult, err
	}
	reconcilationResult = utils.UpdateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.createUser(ctx, tenantControlPlane)
	if err != nil {
		return reconcilationResult, err
	}
	reconcilationResult = utils.UpdateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.createGrantPrivileges(ctx, tenantControlPlane)
	if err != nil {
		return reconcilationResult, err
	}
	reconcilationResult = utils.UpdateOperationResult(reconcilationResult, operationResult)

	return reconcilationResult, nil
}

func (r *Setup) GetName() string {
	return "datastore-setup"
}

func (r *Setup) Delete(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if err := r.Define(ctx, tenantControlPlane); err != nil {
		return err
	}

	if err := r.revokeGrantPrivileges(ctx, tenantControlPlane); err != nil {
		return err
	}

	if err := r.deleteDB(ctx, tenantControlPlane); err != nil {
		return err
	}

	if err := r.deleteUser(ctx, tenantControlPlane); err != nil {
		return err
	}

	return nil
}

func (r *Setup) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Storage.Setup.Schema = r.resource.schema
	tenantControlPlane.Status.Storage.Setup.User = r.resource.user
	tenantControlPlane.Status.Storage.Setup.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Storage.Setup.Checksum = tenantControlPlane.Status.Storage.Config.Checksum

	return nil
}

func (r *Setup) createDB(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	exists, err := r.Connection.DBExists(ctx, r.resource.schema)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if exists {
		return controllerutil.OperationResultNone, nil
	}

	if err := r.Connection.CreateDB(ctx, r.resource.schema); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}

func (r *Setup) deleteDB(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) error {
	exists, err := r.Connection.DBExists(ctx, r.resource.schema)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := r.Connection.DeleteDB(ctx, r.resource.schema); err != nil {
		return err
	}

	return nil
}

func (r *Setup) createUser(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	exists, err := r.Connection.UserExists(ctx, r.resource.user)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if exists {
		return controllerutil.OperationResultNone, nil
	}

	if err := r.Connection.CreateUser(ctx, r.resource.user, r.resource.password); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}

func (r *Setup) deleteUser(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) error {
	exists, err := r.Connection.UserExists(ctx, r.resource.user)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := r.Connection.DeleteUser(ctx, r.resource.user); err != nil {
		return err
	}

	return nil
}

func (r *Setup) createGrantPrivileges(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	exists, err := r.Connection.GrantPrivilegesExists(ctx, r.resource.user, r.resource.schema)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if exists {
		return controllerutil.OperationResultNone, nil
	}

	if err := r.Connection.GrantPrivileges(ctx, r.resource.user, r.resource.schema); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}

func (r *Setup) revokeGrantPrivileges(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) error {
	exists, err := r.Connection.GrantPrivilegesExists(ctx, r.resource.user, r.resource.schema)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := r.Connection.RevokePrivileges(ctx, r.resource.user, r.resource.schema); err != nil {
		return err
	}

	return nil
}
