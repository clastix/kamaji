// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func NewStorageConnection(ctx context.Context, client client.Client, ds kamajiv1alpha1.DataStore) (Connection, error) {
	cc, err := NewConnectionConfig(ctx, client, ds)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection config object: %w", err)
	}

	switch ds.Spec.Driver {
	case kamajiv1alpha1.KineMySQLDriver:

		if ds.Spec.TLSConfig != nil {
			cc.TLSConfig.ServerName = cc.Endpoints[0].Host
		}

		cc.Parameters = map[string][]string{
			"multiStatements": {"true"},
		}

		return NewMySQLConnection(*cc)
	case kamajiv1alpha1.KinePostgreSQLDriver:
		if ds.Spec.TLSConfig != nil {
			cc.TLSConfig.ServerName = cc.Endpoints[0].Host
		}

		return NewPostgreSQLConnection(*cc)
	case kamajiv1alpha1.EtcdDriver:
		return NewETCDConnection(*cc)
	case kamajiv1alpha1.KineNatsDriver:
		return NewNATSConnection(*cc)
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
