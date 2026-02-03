// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"errors"
	"fmt"

	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	etcdclient "go.etcd.io/etcd/client/v3"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	dserrors "github.com/clastix/kamaji/internal/datastore/errors"
)

func NewETCDConnection(config ConnectionConfig) (Connection, error) {
	endpoints := make([]string, 0, len(config.Endpoints))

	for _, ep := range config.Endpoints {
		endpoints = append(endpoints, ep.String())
	}

	cfg := etcdclient.Config{
		Endpoints: endpoints,
		TLS:       config.TLSConfig,
	}

	client, err := etcdclient.New(cfg)
	if err != nil {
		return nil, err
	}

	return &EtcdClient{
		Client: *client,
	}, nil
}

type EtcdClient struct {
	Client etcdclient.Client
}

func (e *EtcdClient) CreateUser(ctx context.Context, user, password string) error {
	if _, err := e.Client.Auth.UserAddWithOptions(ctx, user, password, &etcdclient.UserAddOptions{NoPassword: true}); err != nil {
		return dserrors.NewCreateUserError(err)
	}

	return nil
}

func (e *EtcdClient) CreateDB(context.Context, string) error {
	return nil
}

func (e *EtcdClient) GrantPrivileges(ctx context.Context, user, dbName string) error {
	if _, err := e.Client.Auth.RoleAdd(ctx, dbName); err != nil {
		return dserrors.NewGrantPrivilegesError(err)
	}

	permission := etcdclient.PermissionType(authpb.READWRITE)
	key := e.buildKey(dbName)

	if _, err := e.Client.RoleGrantPermission(ctx, dbName, key, etcdclient.GetPrefixRangeEnd(key), permission); err != nil {
		return dserrors.NewGrantPrivilegesError(err)
	}

	if _, err := e.Client.UserGrantRole(ctx, user, dbName); err != nil {
		return dserrors.NewGrantPrivilegesError(err)
	}

	return nil
}

func (e *EtcdClient) UserExists(ctx context.Context, user string) (bool, error) {
	if _, err := e.Client.UserGet(ctx, user); err != nil {
		if errors.As(err, &rpctypes.ErrGRPCUserNotFound) {
			return false, nil
		}

		return false, dserrors.NewCheckUserExistsError(err)
	}

	return true, nil
}

func (e *EtcdClient) DBExists(context.Context, string) (bool, error) {
	return true, nil
}

func (e *EtcdClient) GrantPrivilegesExists(ctx context.Context, username, dbName string) (bool, error) {
	_, err := e.Client.RoleGet(ctx, dbName)
	if err != nil {
		if errors.As(err, &rpctypes.ErrGRPCRoleNotFound) {
			return false, nil
		}

		return false, dserrors.NewCheckGrantExistsError(err)
	}

	user, err := e.Client.UserGet(ctx, username)
	if err != nil {
		return false, dserrors.NewCheckGrantExistsError(err)
	}

	for _, i := range user.Roles {
		if i == dbName {
			return true, nil
		}
	}

	return false, nil
}

func (e *EtcdClient) DeleteUser(ctx context.Context, user string) error {
	if _, err := e.Client.Auth.UserDelete(ctx, user); err != nil {
		return dserrors.NewDeleteUserError(err)
	}

	return nil
}

func (e *EtcdClient) DeleteDB(ctx context.Context, dbName string) error {
	prefix := e.buildKey(dbName)
	if _, err := e.Client.Delete(ctx, prefix, etcdclient.WithPrefix()); err != nil {
		return dserrors.NewCannotDeleteDatabaseError(err)
	}

	return nil
}

func (e *EtcdClient) RevokePrivileges(ctx context.Context, _, dbName string) error {
	if _, err := e.Client.Auth.RoleDelete(ctx, dbName); err != nil {
		return dserrors.NewRevokePrivilegesError(err)
	}

	return nil
}

func (e *EtcdClient) GetConnectionString() string {
	// There's no need for connection string in etcd client:
	// it's not used by Kine
	return ""
}

func (e *EtcdClient) Close() error {
	if err := e.Client.Close(); err != nil {
		return dserrors.NewCloseConnectionError(err)
	}

	return nil
}

func (e *EtcdClient) Check(ctx context.Context) error {
	if _, err := e.Client.AuthStatus(ctx); err != nil {
		return dserrors.NewCheckConnectionError(err)
	}

	return nil
}

func (e *EtcdClient) Driver() string {
	return string(kamajiv1alpha1.EtcdDriver)
}

// buildKey adds slashes to the beginning and end of the key. This ensures that the range
// end for etcd RBAC is calculated using the entire key prefix, not only the key name. If
// the range end was calculated e.g. for `/cp-a`, the result would be `/cp-b`, which also
// includes `/cp-aa` (etcd uses lexicographic ordering on key ranges for RBAC). Using
// `/cp-a/` as the input for the range end calculation results in `/cp-a0`, which doesn't
// allow for any other potential control plane key prefixes to be located within the range.
// For more information, see also https://etcd.io/docs/v3.3/learning/api/#key-ranges
func (e *EtcdClient) buildKey(key string) string {
	return fmt.Sprintf("/%s/", key)
}

func (e *EtcdClient) Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error {
	targetClient := target.(*EtcdClient) //nolint:forcetypeassert

	if err := target.Check(ctx); err != nil {
		return err
	}

	response, err := e.Client.Get(ctx, e.buildKey(tcp.Status.Storage.Setup.Schema), etcdclient.WithPrefix())
	if err != nil {
		return err
	}

	for _, kv := range response.Kvs {
		if _, err = targetClient.Client.Put(ctx, string(kv.Key), string(kv.Value)); err != nil {
			return err
		}
	}

	return nil
}
