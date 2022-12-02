// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func NewStorageConnection(ctx context.Context, client client.Client, ds kamajiv1alpha1.DataStore) (Connection, error) {
	cc, err := NewConnectionConfig(ctx, client, ds)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create connection config object")
	}

	switch ds.Spec.Driver {
	case kamajiv1alpha1.KineMySQLDriver:
		cc.TLSConfig.ServerName = cc.Endpoints[0].Host
		cc.Parameters = map[string][]string{
			"multiStatements": {"true"},
		}

		return NewMySQLConnection(*cc)
	case kamajiv1alpha1.KinePostgreSQLDriver:
		cc.TLSConfig.ServerName = cc.Endpoints[0].Host
		//nolint:contextcheck
		return NewPostgreSQLConnection(*cc)
	case kamajiv1alpha1.EtcdDriver:
		return NewETCDConnection(*cc)
	default:
		return nil, fmt.Errorf("%s is not a valid driver", ds.Spec.Driver)
	}
}

type Connection interface {
	CreateUser(ctx context.Context, user, password string) error
	CreateDB(ctx context.Context, dbName string) error
	GrantPrivileges(ctx context.Context, user, dbName string) error
	UserExists(ctx context.Context, user string) (bool, error)
	DBExists(ctx context.Context, dbName string) (bool, error)
	GrantPrivilegesExists(ctx context.Context, user, dbName string) (bool, error)
	DeleteUser(ctx context.Context, user string) error
	DeleteDB(ctx context.Context, dbName string) error
	RevokePrivileges(ctx context.Context, user, dbName string) error
	GetConnectionString() string
	Close() error
	Check(ctx context.Context) error
	Driver() string
	Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error
}
