// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	etcdclient "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
)

const (
	etcdTimeout = 10 // seconds

	//  rangeEnd is the key following the last key of the range.
	//  If rangeEnd is ‘\0’, the range is all keys greater than or equal to the key argument
	//  source: https://etcd.io/docs/v3.5/learning/api/
	rangeEnd = "\\0"
)

func NewClient(config Config) (*etcdclient.Client, error) {
	cert, err := tls.X509KeyPair(config.ETCDCertificate, config.ETCDPrivateKey)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(config.ETCDCA)

	cfg := etcdclient.Config{
		Endpoints: config.Endpoints,
		TLS: &tls.Config{ // nolint:gosec
			Certificates: []tls.Certificate{cert},
			RootCAs:      pool,
		},
	}

	return etcdclient.New(cfg)
}

func GetUser(ctx context.Context, client *etcdclient.Client, user *User) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, etcdTimeout*time.Second)
	defer cancel()

	response, err := client.UserGet(ctxWithTimeout, user.Name)
	if err != nil {
		var etcdError rpctypes.EtcdError
		if errors.As(err, &etcdError) && etcdError.Code() == codes.FailedPrecondition {
			return nil
		}

		return err
	}

	user.Roles = response.Roles
	user.Exists = true

	return nil
}

func AddUser(ctx context.Context, client *etcdclient.Client, username string) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()
	opts := etcdclient.UserAddOptions{
		NoPassword: true,
	}
	_, err := client.Auth.UserAddWithOptions(ctxWithTimeout, username, "", &opts)

	return err
}

func RemoveUser(ctx context.Context, client *etcdclient.Client, username string) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	_, err := client.Auth.UserDelete(ctxWithTimeout, username)

	return err
}

func GetRole(ctx context.Context, client *etcdclient.Client, role *Role) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	response, err := client.RoleGet(ctxWithTimeout, role.Name)
	if err != nil {
		var etcdError rpctypes.EtcdError
		if errors.As(err, &etcdError) && etcdError.Code() == codes.FailedPrecondition {
			return nil
		}

		return err
	}

	role.Exists = true
	for _, perm := range response.Perm {
		permission := Permission{
			Type:     int(perm.PermType),
			Key:      string(perm.Key),
			RangeEnd: string(perm.RangeEnd),
		}
		role.Permissions = append(role.Permissions, permission)
	}

	return nil
}

func AddRole(ctx context.Context, client *etcdclient.Client, roleName string) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	_, err := client.Auth.RoleAdd(ctxWithTimeout, roleName)

	return err
}

func RemoveRole(ctx context.Context, client *etcdclient.Client, roleName string) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	_, err := client.Auth.RoleDelete(ctxWithTimeout, roleName)

	return err
}

func GrantUserRole(ctx context.Context, client *etcdclient.Client, user User, role Role) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	_, err := client.UserGrantRole(ctxWithTimeout, user.Name, role.Name)
	if err != nil {
		return err
	}

	return nil
}

func GrantRolePermission(ctx context.Context, client *etcdclient.Client, role Role) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	permission := etcdclient.PermissionType(authpb.READWRITE)
	key := BuildKey(role.Name)
	_, err := client.RoleGrantPermission(ctxWithTimeout, role.Name, key, rangeEnd, permission)
	if err != nil {
		return err
	}

	return nil
}

func CleanUpPrefix(ctx context.Context, client *etcdclient.Client, name string) error {
	ctxWithTimeout, cancel := getContextWithTimeout(ctx)
	defer cancel()

	withRange := etcdclient.WithRange(rangeEnd)
	prefix := BuildKey(name)
	_, err := client.Delete(ctxWithTimeout, prefix, withRange)

	return err
}

func BuildKey(roleName string) string {
	return fmt.Sprintf("/%s/", roleName)
}

func getContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, etcdTimeout*time.Second)
}
