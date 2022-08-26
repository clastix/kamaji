// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	etcdclient "go.etcd.io/etcd/client/v3"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
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
	_, err := e.Client.Auth.UserAddWithOptions(ctx, user, password, &etcdclient.UserAddOptions{
		NoPassword: true,
	})

	return err
}

func (e *EtcdClient) CreateDB(ctx context.Context, dbName string) error {
	return nil
}

func (e *EtcdClient) GrantPrivileges(ctx context.Context, user, dbName string) error {
	_, err := e.Client.Auth.RoleAdd(ctx, dbName)
	if err != nil {
		return err
	}

	permission := etcdclient.PermissionType(authpb.READWRITE)
	key := e.buildKey(dbName)
	if _, err = e.Client.RoleGrantPermission(ctx, user, key, rangeEnd, permission); err != nil {
		return err
	}

	if _, err = e.Client.UserGrantRole(ctx, user, dbName); err != nil {
		return err
	}

	return err
}

func (e *EtcdClient) UserExists(ctx context.Context, user string) (bool, error) {
	_, err := e.Client.UserGet(ctx, user)
	if err != nil {
		if errors.As(err, &rpctypes.ErrGRPCUserNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (e *EtcdClient) DBExists(_ context.Context, dbName string) (bool, error) {
	return true, nil
}

func (e *EtcdClient) GrantPrivilegesExists(ctx context.Context, username, dbName string) (bool, error) {
	_, err := e.Client.RoleGet(ctx, dbName)
	if err != nil {
		if errors.As(err, &rpctypes.ErrGRPCRoleNotFound) {
			return false, nil
		}

		return false, err
	}

	user, err := e.Client.UserGet(ctx, username)
	if err != nil {
		return false, err
	}

	for _, i := range user.Roles {
		if i == dbName {
			return true, nil
		}
	}

	return false, nil
}

func (e *EtcdClient) DeleteUser(ctx context.Context, user string) error {
	_, err := e.Client.Auth.UserDelete(ctx, user)

	return err
}

func (e *EtcdClient) DeleteDB(ctx context.Context, dbName string) error {
	withRange := etcdclient.WithRange(rangeEnd)
	prefix := e.buildKey(dbName)
	_, err := e.Client.Delete(ctx, prefix, withRange)

	return err
}

func (e *EtcdClient) RevokePrivileges(ctx context.Context, user, dbName string) error {
	_, err := e.Client.Auth.RoleDelete(ctx, dbName)

	return err
}

func (e *EtcdClient) GetConnectionString() string {
	// There's no need for connection string in etcd client:
	// it's not used by Kine
	return ""
}

func (e *EtcdClient) Close() error {
	return e.Client.Close()
}

func (e *EtcdClient) Check(ctx context.Context) error {
	_, err := e.Client.AuthStatus(ctx)

	return err
}

func (e *EtcdClient) Driver() string {
	return string(kamajiv1alpha1.EtcdDriver)
}

func (e *EtcdClient) buildKey(roleName string) string {
	return fmt.Sprintf("/%s/", roleName)
}

type Permission struct {
	Type     int    `json:"type,omitempty"`
	Key      string `json:"key,omitempty"`
	RangeEnd string `json:"rangeEnd,omitempty"`
}
