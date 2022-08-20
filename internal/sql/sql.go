// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
)

type Driver int

const (
	MySQL Driver = iota
	PostgreSQL
)

func (d Driver) ToString() string {
	switch d {
	case MySQL:
		return "mysql"
	case PostgreSQL:
		return "postgresql"
	default:
		return ""
	}
}

type ConnectionConfig struct {
	SQLDriver  Driver
	User       string
	Password   string
	Host       string
	Port       int
	DBName     string
	TLSConfig  *tls.Config
	Parameters map[string][]string
}

func (config ConnectionConfig) GetDataSourceName() string {
	userPassword := config.getDataSourceNameUserPassword()
	db := config.getDataSourceNameDB()
	dataSourceName := fmt.Sprintf("%s%s/%s?%s", userPassword, db, config.DBName, config.formatParameters())

	return dataSourceName
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

func (config ConnectionConfig) getDataSourceNameDB() string {
	if config.Host == "" || config.Port < firstPort {
		return ""
	}

	return fmt.Sprintf("%s(%s:%d)", defaultProtocol, config.Host, config.Port)
}

func (config ConnectionConfig) formatParameters() string {
	if len(config.Parameters) == 0 {
		return ""
	}

	values := url.Values(config.Parameters)

	return values.Encode()
}

type DBConnection interface {
	CreateUser(ctx context.Context, user, password string) error
	CreateDB(ctx context.Context, dbName string) error
	GrantPrivileges(ctx context.Context, user, dbName string) error
	UserExists(ctx context.Context, user string) (bool, error)
	DBExists(ctx context.Context, dbName string) (bool, error)
	GrantPrivilegesExists(ctx context.Context, user, dbName string) (bool, error)
	DeleteUser(ctx context.Context, user string) error
	DeleteDB(ctx context.Context, dbName string) error
	RevokePrivileges(ctx context.Context, user, dbName string) error
	GetHost() string
	GetPort() int
	Close() error
	Check() error
	Driver() string
}

func GetDBConnection(config ConnectionConfig) (DBConnection, error) {
	switch config.SQLDriver {
	case MySQL:
		return getMySQLDB(config)
	case PostgreSQL:
		return getPostgreSQLDB(config)
	default:
		return nil, fmt.Errorf("%s is not a valid driver", config.SQLDriver.ToString())
	}
}

func checkEmptyQueryResult(err error) bool {
	return err.Error() == sqlErrorNoRows
}
