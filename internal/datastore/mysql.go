// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"github.com/go-pg/pg/v10"
	"github.com/go-sql-driver/mysql"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

const (
	defaultProtocol = "tcp"
	sqlErrorNoRows  = "sql: no rows in result set"
)

const (
	mysqlFetchUserStatement        = "SELECT User FROM mysql.user WHERE User= ? LIMIT 1"
	mysqlFetchDBStatement          = "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME= ? LIMIT 1"
	mysqlShowGrantsStatement       = "SHOW GRANTS FOR `%s`@`%%`"
	mysqlCreateDBStatement         = "CREATE DATABASE IF NOT EXISTS %s"
	mysqlCreateUserStatement       = "CREATE USER `%s`@`%%` IDENTIFIED BY '%s'"
	mysqlGrantPrivilegesStatement  = "GRANT ALL PRIVILEGES ON `%s`.* TO `%s`@`%%`"
	mysqlDropDBStatement           = "DROP DATABASE IF EXISTS `%s`"
	mysqlDropUserStatement         = "DROP USER IF EXISTS `%s`"
	mysqlRevokePrivilegesStatement = "REVOKE ALL PRIVILEGES ON `%s`.* FROM `%s`"
)

type MySQLConnection struct {
	db        *sql.DB
	connector ConnectionEndpoint
}

func (c *MySQLConnection) Driver() string {
	return string(kamajiv1alpha1.KineMySQLDriver)
}

func NewPostgreSQLConnection(config ConnectionConfig) (Connection, error) {
	opt := &pg.Options{
		Addr:      config.Endpoints[0].String(),
		Database:  config.DBName,
		User:      config.User,
		Password:  config.Password,
		TLSConfig: config.TLSConfig,
	}

	return &PostgreSQLConnection{db: pg.Connect(opt), connection: config.Endpoints[0]}, nil
}

func NewMySQLConnection(config ConnectionConfig) (Connection, error) {
	nameDB := fmt.Sprintf("%s(%s)", defaultProtocol, config.Endpoints[0].String())

	var parameters string
	if len(config.Parameters) > 0 {
		parameters = url.Values(config.Parameters).Encode()
	}

	dsn := fmt.Sprintf("%s%s/%s?%s", config.getDataSourceNameUserPassword(), nameDB, config.DBName, parameters)

	mysqlConfig, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	tlsKey := "mysql"

	if err = mysql.RegisterTLSConfig(tlsKey, config.TLSConfig); err != nil {
		return nil, err
	}

	mysqlConfig.DBName = config.DBName
	mysqlConfig.TLSConfig = tlsKey
	parsedDSN := mysqlConfig.FormatDSN()

	db, err := sql.Open("mysql", parsedDSN)
	if err != nil {
		return nil, err
	}

	return &MySQLConnection{db: db, connector: config.Endpoints[0]}, nil
}

func (c *MySQLConnection) GetConnectionString() string {
	return c.connector.String()
}

func (c *MySQLConnection) Close() error {
	return c.db.Close()
}

func (c *MySQLConnection) Check(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *MySQLConnection) CreateUser(ctx context.Context, user, password string) error {
	return c.mutate(ctx, mysqlCreateUserStatement, user, password)
}

func (c *MySQLConnection) CreateDB(ctx context.Context, dbName string) error {
	return c.mutate(ctx, mysqlCreateDBStatement, dbName)
}

func (c *MySQLConnection) GrantPrivileges(ctx context.Context, user, dbName string) error {
	return c.mutate(ctx, mysqlGrantPrivilegesStatement, user, dbName)
}

func (c *MySQLConnection) UserExists(ctx context.Context, user string) (bool, error) {
	checker := func(row *sql.Row) (bool, error) {
		var name string
		if err := row.Scan(&name); err != nil {
			if c.checkEmptyQueryResult(err) {
				return false, nil
			}

			return false, err
		}

		return name == user, nil
	}

	return c.check(ctx, mysqlFetchUserStatement, checker, user)
}

func (c *MySQLConnection) DBExists(ctx context.Context, dbName string) (bool, error) {
	checker := func(row *sql.Row) (bool, error) {
		var name string
		if err := row.Scan(&name); err != nil {
			if c.checkEmptyQueryResult(err) {
				return false, nil
			}

			return false, err
		}

		return name == dbName, nil
	}

	return c.check(ctx, mysqlFetchDBStatement, checker, dbName)
}

func (c *MySQLConnection) GrantPrivilegesExists(ctx context.Context, user, dbName string) (bool, error) {
	statementShowGrantsStatement := fmt.Sprintf(mysqlShowGrantsStatement, user)
	rows, err := c.db.Query(statementShowGrantsStatement)
	if err != nil {
		return false, err
	}

	expected := fmt.Sprintf(mysqlGrantPrivilegesStatement, user, dbName)
	var grant string

	for rows.Next() {
		if err = rows.Scan(&grant); err != nil {
			return false, err
		}

		if grant == expected {
			return true, nil
		}
	}

	return false, nil
}

func (c *MySQLConnection) DeleteUser(ctx context.Context, user string) error {
	return c.mutate(ctx, mysqlDropUserStatement, user)
}

func (c *MySQLConnection) DeleteDB(ctx context.Context, dbName string) error {
	return c.mutate(ctx, mysqlDropDBStatement, dbName)
}

func (c *MySQLConnection) RevokePrivileges(ctx context.Context, user, dbName string) error {
	return c.mutate(ctx, mysqlRevokePrivilegesStatement, user, dbName)
}

func (c *MySQLConnection) check(ctx context.Context, nonFilledStatement string, checker func(*sql.Row) (bool, error), args ...any) (bool, error) {
	statement, err := c.db.Prepare(nonFilledStatement)
	if err != nil {
		return false, err
	}
	defer statement.Close()

	row := statement.QueryRowContext(ctx, args...)

	return checker(row)
}

func (c *MySQLConnection) mutate(ctx context.Context, nonFilledStatement string, args ...any) error {
	statement := fmt.Sprintf(nonFilledStatement, args...)
	if _, err := c.db.ExecContext(ctx, statement); err != nil {
		return err
	}

	return nil
}

func (c *MySQLConnection) checkEmptyQueryResult(err error) bool {
	return err.Error() == sqlErrorNoRows
}
