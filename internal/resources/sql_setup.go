// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/sql"
)

const (
	secretHashLabelKey = "component.kamaji.clastix.io/secret-hash"
)

type sqlSetupResource struct {
	schema   string
	user     string
	password string
}

type SQLSetup struct {
	resource     sqlSetupResource
	Client       client.Client
	DBConnection sql.DBConnection
	Name         string
}

func (r *SQLSetup) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Storage.KineMySQL == nil {
		return true
	}

	return tenantControlPlane.Status.Storage.KineMySQL.Setup.Schema != r.resource.schema ||
		tenantControlPlane.Status.Storage.KineMySQL.Setup.User != r.resource.user ||
		tenantControlPlane.Status.Storage.KineMySQL.Setup.SQLConfigResourceVersion != tenantControlPlane.Status.Storage.KineMySQL.Config.ResourceVersion
}

func (r *SQLSetup) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *SQLSetup) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *SQLSetup) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: tenantControlPlane.GetNamespace(),
		Name:      tenantControlPlane.Status.Storage.KineMySQL.Config.SecretName,
	}
	if err := r.Client.Get(ctx, namespacedName, secret); err != nil {
		return err
	}

	r.resource = sqlSetupResource{
		schema:   string(secret.Data["MYSQL_SCHEMA"]),
		user:     string(secret.Data["MYSQL_USER"]),
		password: string(secret.Data["MYSQL_PASSWORD"]),
	}

	return nil
}

func (r *SQLSetup) GetClient() client.Client {
	return r.Client
}

func (r *SQLSetup) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tenantControlPlane.Status.Storage.KineMySQL.Setup.SQLConfigResourceVersion != "" &&
		tenantControlPlane.Status.Storage.KineMySQL.Setup.SQLConfigResourceVersion != tenantControlPlane.Status.Storage.KineMySQL.Config.ResourceVersion {
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
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.createUser(ctx, tenantControlPlane)
	if err != nil {
		return reconcilationResult, err
	}
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.createGrantPrivileges(ctx, tenantControlPlane)
	if err != nil {
		return reconcilationResult, err
	}
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	return reconcilationResult, nil
}

func (r *SQLSetup) GetName() string {
	return r.Name
}

func (r *SQLSetup) Delete(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
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

func (r *SQLSetup) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Storage.KineMySQL == nil {
		return fmt.Errorf("sql configuration is not ready")
	}

	tenantControlPlane.Status.Storage.KineMySQL.Setup.Schema = r.resource.schema
	tenantControlPlane.Status.Storage.KineMySQL.Setup.User = r.resource.user
	tenantControlPlane.Status.Storage.KineMySQL.Setup.LastUpdate = metav1.Now()
	tenantControlPlane.Status.Storage.KineMySQL.Setup.SQLConfigResourceVersion = tenantControlPlane.Status.Storage.KineMySQL.Config.ResourceVersion

	return nil
}

func (r *SQLSetup) createDB(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	exists, err := r.DBConnection.DBExists(ctx, r.resource.schema)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if exists {
		return controllerutil.OperationResultNone, nil
	}

	if err := r.DBConnection.CreateDB(ctx, r.resource.schema); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}

func (r *SQLSetup) deleteDB(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	exists, err := r.DBConnection.DBExists(ctx, r.resource.schema)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := r.DBConnection.DeleteDB(ctx, r.resource.schema); err != nil {
		return err
	}

	return nil
}

func (r *SQLSetup) createUser(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	exists, err := r.DBConnection.UserExists(ctx, r.resource.user)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if exists {
		return controllerutil.OperationResultNone, nil
	}

	if err := r.DBConnection.CreateUser(ctx, r.resource.user, r.resource.password); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}

func (r *SQLSetup) deleteUser(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	exists, err := r.DBConnection.UserExists(ctx, r.resource.user)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := r.DBConnection.DeleteUser(ctx, r.resource.user); err != nil {
		return err
	}

	return nil
}

func (r *SQLSetup) createGrantPrivileges(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	exists, err := r.DBConnection.GrantPrivilegesExists(ctx, r.resource.user, r.resource.schema)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if exists {
		return controllerutil.OperationResultNone, nil
	}

	if err := r.DBConnection.GrantPrivileges(ctx, r.resource.user, r.resource.schema); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}

func (r *SQLSetup) revokeGrantPrivileges(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	exists, err := r.DBConnection.GrantPrivilegesExists(ctx, r.resource.user, r.resource.schema)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := r.DBConnection.RevokePrivileges(ctx, r.resource.user, r.resource.schema); err != nil {
		return err
	}

	return nil
}
