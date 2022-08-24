// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/go-logr/logr"
	etcdclient "go.etcd.io/etcd/client/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/etcd"
)

type etcdSetupResource struct {
	role etcd.Role
	user etcd.User
}

type ETCDSetupResource struct {
	resource  *etcdSetupResource
	Client    client.Client
	Log       logr.Logger
	DataStore kamajiv1alpha1.DataStore
}

func (r *ETCDSetupResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Storage.ETCD == nil {
		return true
	}

	return tenantControlPlane.Status.Storage.ETCD.Role.Name != r.resource.role.Name ||
		tenantControlPlane.Status.Storage.ETCD.User.Name != r.resource.user.Name
}

func (r *ETCDSetupResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *ETCDSetupResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *ETCDSetupResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &etcdSetupResource{
		role: etcd.Role{Name: tenantControlPlane.Name, Exists: false},
		user: etcd.User{Name: tenantControlPlane.Name, Exists: false},
	}

	return nil
}

func (r *ETCDSetupResource) CreateOrUpdate(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return r.reconcile(ctx)
}

func (r *ETCDSetupResource) Delete(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if err := r.Define(ctx, tenantControlPlane); err != nil {
		return err
	}

	client, err := r.getETCDClient(ctx)
	if err != nil {
		return err
	}

	if err = r.deleteData(ctx, client, tenantControlPlane); err != nil {
		return err
	}

	if err = r.deleteUser(ctx, client, tenantControlPlane); err != nil {
		return err
	}

	if err = r.deleteRole(ctx, client, tenantControlPlane); err != nil {
		return err
	}

	return nil
}

func (r *ETCDSetupResource) GetName() string {
	return "etcd-setup"
}

func (r *ETCDSetupResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Status.Storage.ETCD == nil {
		tenantControlPlane.Status.Storage.ETCD = &kamajiv1alpha1.ETCDStatus{}
	}
	tenantControlPlane.Status.Storage.ETCD.Role = r.resource.role
	tenantControlPlane.Status.Storage.ETCD.User = r.resource.user

	return nil
}

func (r *ETCDSetupResource) reconcile(ctx context.Context) (controllerutil.OperationResult, error) {
	reconcilationResult := controllerutil.OperationResultNone
	var operationResult controllerutil.OperationResult

	client, err := r.getETCDClient(ctx)
	if err != nil {
		return reconcilationResult, err
	}
	defer client.Close()

	operationResult, err = r.reconcileUser(ctx, client)
	if err != nil {
		return reconcilationResult, err
	}
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.reconcileRole(ctx, client)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.grantUserRole(ctx, client)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	operationResult, err = r.grantRolePermissions(ctx, client)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	reconcilationResult = updateOperationResult(reconcilationResult, operationResult)

	return reconcilationResult, nil
}

func (r *ETCDSetupResource) getETCDClient(ctx context.Context) (*etcdclient.Client, error) {
	ca, err := r.DataStore.Spec.TLSConfig.CertificateAuthority.Certificate.GetContent(ctx, r.Client)
	if err != nil {
		return nil, err
	}

	crt, err := r.DataStore.Spec.TLSConfig.ClientCertificate.Certificate.GetContent(ctx, r.Client)
	if err != nil {
		return nil, err
	}

	key, err := r.DataStore.Spec.TLSConfig.ClientCertificate.PrivateKey.GetContent(ctx, r.Client)
	if err != nil {
		return nil, err
	}

	config := etcd.Config{
		ETCDCertificate: crt,
		ETCDPrivateKey:  key,
		ETCDCA:          ca,
		Endpoints:       r.DataStore.Spec.Endpoints,
	}

	return etcd.NewClient(config)
}

func (r *ETCDSetupResource) reconcileUser(ctx context.Context, client *etcdclient.Client) (controllerutil.OperationResult, error) {
	if err := etcd.GetUser(ctx, client, &r.resource.user); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if !r.resource.user.Exists {
		if err := etcd.AddUser(ctx, client, r.resource.user.Name); err != nil {
			return controllerutil.OperationResultNone, err
		}

		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultNone, nil
}

func (r *ETCDSetupResource) reconcileRole(ctx context.Context, client *etcdclient.Client) (controllerutil.OperationResult, error) {
	if err := etcd.GetRole(ctx, client, &r.resource.role); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if !r.resource.role.Exists {
		if err := etcd.AddRole(ctx, client, r.resource.role.Name); err != nil {
			return controllerutil.OperationResultNone, err
		}

		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultNone, nil
}

func (r *ETCDSetupResource) grantUserRole(ctx context.Context, client *etcdclient.Client) (controllerutil.OperationResult, error) {
	if err := etcd.GetUser(ctx, client, &r.resource.user); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if len(r.resource.user.Roles) > 0 && isRole(r.resource.user.Roles, r.resource.role.Name) {
		return controllerutil.OperationResultNone, nil
	}

	if err := etcd.GrantUserRole(ctx, client, r.resource.user, r.resource.role); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultUpdated, nil
}

func (r *ETCDSetupResource) grantRolePermissions(ctx context.Context, client *etcdclient.Client) (controllerutil.OperationResult, error) {
	if err := etcd.GetRole(ctx, client, &r.resource.role); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if len(r.resource.role.Permissions) > 0 && isPermission(r.resource.role.Permissions, r.resource.role.Name) {
		return controllerutil.OperationResultNone, nil
	}

	if err := etcd.GrantRolePermission(ctx, client, r.resource.role); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultUpdated, nil
}

func (r *ETCDSetupResource) deleteData(ctx context.Context, client *etcdclient.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	return etcd.CleanUpPrefix(ctx, client, tenantControlPlane.GetName())
}

func (r *ETCDSetupResource) deleteUser(ctx context.Context, client *etcdclient.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if err := etcd.GetUser(ctx, client, &r.resource.user); err != nil {
		return err
	}

	if !r.resource.user.Exists {
		return nil
	}

	return etcd.RemoveUser(ctx, client, tenantControlPlane.GetName())
}

func (r *ETCDSetupResource) deleteRole(ctx context.Context, client *etcdclient.Client, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if err := etcd.GetRole(ctx, client, &r.resource.role); err != nil {
		return err
	}

	if !r.resource.role.Exists {
		return nil
	}

	return etcd.RemoveRole(ctx, client, tenantControlPlane.GetName())
}

func isRole(s []string, x string) bool {
	for _, o := range s {
		if o == x {
			return true
		}
	}

	return false
}

func isPermission(s []etcd.Permission, role string) bool {
	key := etcd.BuildKey(role)
	for _, o := range s {
		if o.Key == key {
			return true
		}
	}

	return false
}
