// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"

	goerrors "github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	etcdclient "go.etcd.io/etcd/client/v3"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/datastore/errors"
)

const (
	// rangeEnd is the key following the last key of the range.
	// If rangeEnd is ‘\0’, the range is all keys greater than or equal to the key argument
	// source: https://etcd.io/docs/v3.5/learning/api/
	rangeEnd = "\\0"
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
		return errors.NewCreateUserError(err)
	}

	return nil
}

func (e *EtcdClient) CreateDB(context.Context, string) error {
	return nil
}

func (e *EtcdClient) GrantPrivileges(ctx context.Context, user, dbName string) error {
	if _, err := e.Client.Auth.RoleAdd(ctx, dbName); err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	permission := etcdclient.PermissionType(authpb.READWRITE)
	key := e.buildKey(dbName)
	if _, err := e.Client.RoleGrantPermission(ctx, user, key, rangeEnd, permission); err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	if _, err := e.Client.UserGrantRole(ctx, user, dbName); err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	return nil
}

func (e *EtcdClient) UserExists(ctx context.Context, user string) (bool, error) {
	if _, err := e.Client.UserGet(ctx, user); err != nil {
		if goerrors.As(err, &rpctypes.ErrGRPCUserNotFound) {
			return false, nil
		}

		return false, errors.NewCheckUserExistsError(err)
	}

	return true, nil
}

func (e *EtcdClient) DBExists(context.Context, string) (bool, error) {
	return true, nil
}

func (e *EtcdClient) GrantPrivilegesExists(ctx context.Context, username, dbName string) (bool, error) {
	_, err := e.Client.RoleGet(ctx, dbName)
	if err != nil {
		if goerrors.As(err, &rpctypes.ErrGRPCRoleNotFound) {
			return false, nil
		}

		return false, errors.NewCheckGrantExistsError(err)
	}

	user, err := e.Client.UserGet(ctx, username)
	if err != nil {
		return false, errors.NewCheckGrantExistsError(err)
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
		return errors.NewDeleteUserError(err)
	}

	return nil
}

func (e *EtcdClient) DeleteDB(ctx context.Context, dbName string) error {
	prefix := e.buildKey(dbName)
	if _, err := e.Client.Delete(ctx, prefix, etcdclient.WithPrefix()); err != nil {
		return errors.NewCannotDeleteDatabaseError(err)
	}

	return nil
}

func (e *EtcdClient) RevokePrivileges(ctx context.Context, _, dbName string) error {
	if _, err := e.Client.Auth.RoleDelete(ctx, dbName); err != nil {
		return errors.NewRevokePrivilegesError(err)
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
		return errors.NewCloseConnectionError(err)
	}

	return nil
}

func (e *EtcdClient) Check(ctx context.Context) error {
	if _, err := e.Client.AuthStatus(ctx); err != nil {
		return errors.NewCheckConnectionError(err)
	}

	return nil
}

func (e *EtcdClient) Driver() string {
	return string(kamajiv1alpha1.EtcdDriver)
}

func (e *EtcdClient) buildKey(key string) string {
	return fmt.Sprintf("/%s/", key)
}

func (e *EtcdClient) Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error {
	targetClient := target.(*EtcdClient) //nolint:forcetypeassert

	if err := target.Check(ctx); err != nil {
		return err
	}

	response, err := e.Client.Get(ctx, e.buildKey(fmt.Sprintf("%s_%s", tcp.GetNamespace(), tcp.GetName())), etcdclient.WithRange(rangeEnd))
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
