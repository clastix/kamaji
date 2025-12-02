// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/JamesStewy/go-mysqldump"
	"github.com/go-sql-driver/mysql"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/datastore/errors"
)

const (
	defaultProtocol = "tcp"
	sqlErrorNoRows  = "sql: no rows in result set"
)

const (
	mysqlFetchUserStatement        = "SELECT User FROM mysql.user WHERE User= ? LIMIT 1"
	mysqlFetchDBStatement          = "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME=? LIMIT 1"
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

func (c *MySQLConnection) Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error {
	// Ensuring the connection is working as expected
	if err := target.Check(ctx); err != nil {
		return err
	}
	// Creating the target schema if it doesn't exist
	if ok, _ := target.DBExists(ctx, tcp.Status.Storage.Setup.Schema); !ok {
		if err := target.CreateDB(ctx, tcp.Status.Storage.Setup.Schema); err != nil {
			return err
		}
	}
	// Dumping the old datastore in a local file
	dir, err := os.MkdirTemp("", string(tcp.GetUID()))
	if err != nil {
		return fmt.Errorf("unable to create temp directory for MySQL migration: %w", err)
	}
	defer os.RemoveAll(dir)

	if _, err = c.db.ExecContext(ctx, fmt.Sprintf("USE %s_%s", tcp.GetNamespace(), tcp.GetName())); err != nil {
		return fmt.Errorf("unable to switch DB for MySQL migration: %w", err)
	}

	dumper, err := mysqldump.Register(c.db, dir, fmt.Sprintf("%d", time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("unable to create MySQL dumper: %w", err)
	}
	defer dumper.Close()

	dumpFile, err := dumper.Dump()
	if err != nil {
		return fmt.Errorf("unable to dump from MySQL: %w", err)
	}

	statements, err := os.ReadFile(dumpFile)
	if err != nil {
		return fmt.Errorf("cannot read dump file for MySQL: %w", err)
	}
	// Executing the import to the target datastore
	targetClient := target.(*MySQLConnection) //nolint:forcetypeassert

	if _, err = targetClient.db.ExecContext(ctx, fmt.Sprintf("USE %s_%s", tcp.GetNamespace(), tcp.GetName())); err != nil {
		return fmt.Errorf("unable to switch DB for MySQL migration: %w", err)
	}

	if _, err = targetClient.db.ExecContext(ctx, string(statements)); err != nil {
		return fmt.Errorf("cannot execute dump statements for MySQL: %w", err)
	}

	return nil
}

func (c *MySQLConnection) Driver() string {
	return string(kamajiv1alpha1.KineMySQLDriver)
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

	if config.TLSConfig != nil {
		if err = mysql.RegisterTLSConfig(tlsKey, config.TLSConfig); err != nil {
			return nil, err
		}
		mysqlConfig.TLSConfig = tlsKey
	}

	mysqlConfig.DBName = config.DBName
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
	if err := c.db.Close(); err != nil {
		return errors.NewCloseConnectionError(err)
	}

	return nil
}

func (c *MySQLConnection) Check(ctx context.Context) error {
	if err := c.db.PingContext(ctx); err != nil {
		return errors.NewCheckConnectionError(err)
	}

	return nil
}

func (c *MySQLConnection) CreateUser(ctx context.Context, user, password string) error {
	if err := c.mutate(ctx, mysqlCreateUserStatement, user, password); err != nil {
		return errors.NewCreateUserError(err)
	}

	return nil
}

func (c *MySQLConnection) CreateDB(ctx context.Context, dbName string) error {
	if err := c.mutate(ctx, mysqlCreateDBStatement, dbName); err != nil {
		return errors.NewCreateDBError(err)
	}

	return nil
}

func (c *MySQLConnection) GrantPrivileges(ctx context.Context, user, dbName string) error {
	if err := c.mutate(ctx, mysqlGrantPrivilegesStatement, dbName, user); err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	return nil
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

	ok, err := c.check(ctx, mysqlFetchUserStatement, checker, user)
	if err != nil {
		return false, errors.NewCheckUserExistsError(err)
	}

	return ok, nil
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

	ok, err := c.check(ctx, mysqlFetchDBStatement, checker, dbName)
	if err != nil {
		return false, errors.NewCheckDatabaseExistError(err)
	}

	return ok, nil
}

func (c *MySQLConnection) GrantPrivilegesExists(_ context.Context, user, dbName string) (bool, error) {
	statementShowGrantsStatement := fmt.Sprintf(mysqlShowGrantsStatement, user)
	rows, err := c.db.Query(statementShowGrantsStatement) //nolint:sqlclosecheck
	if err != nil {
		return false, errors.NewGrantPrivilegesError(err)
	}

	if err = rows.Err(); err != nil {
		return false, errors.NewGrantPrivilegesError(err)
	}

	expected := fmt.Sprintf(mysqlGrantPrivilegesStatement, dbName, user)
	var grant string

	for rows.Next() {
		if err = rows.Scan(&grant); err != nil {
			return false, errors.NewGrantPrivilegesError(err)
		}

		if grant == expected {
			return true, nil
		}
	}

	return false, nil
}

func (c *MySQLConnection) DeleteUser(ctx context.Context, user string) error {
	if err := c.mutate(ctx, mysqlDropUserStatement, user); err != nil {
		return errors.NewDeleteUserError(err)
	}

	return nil
}

func (c *MySQLConnection) DeleteDB(ctx context.Context, dbName string) error {
	if err := c.mutate(ctx, mysqlDropDBStatement, dbName); err != nil {
		return errors.NewCannotDeleteDatabaseError(err)
	}

	return nil
}

func (c *MySQLConnection) RevokePrivileges(ctx context.Context, user, dbName string) error {
	if err := c.mutate(ctx, mysqlRevokePrivilegesStatement, dbName, user); err != nil {
		return errors.NewRevokePrivilegesError(err)
	}

	return nil
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
