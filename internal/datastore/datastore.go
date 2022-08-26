// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"crypto/tls"
	"fmt"
)

type ConnectionEndpoint struct {
	Host string
	Port int
}

func (r ConnectionEndpoint) String() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type ConnectionConfig struct {
	User       string
	Password   string
	Endpoints  []ConnectionEndpoint
	DBName     string
	TLSConfig  *tls.Config
	Parameters map[string][]string
}

func (config ConnectionConfig) getDataSourceNameUserPassword() string {
	if config.User == "" {
		return ""
	}

	if config.Password == "" {
		return fmt.Sprintf("%s@", config.User)
	}

	return fmt.Sprintf("%s:%s@", config.User, config.Password)
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
}
