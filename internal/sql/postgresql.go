// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-pg/pg/v10"
)

const (
	postgresqlFetchDBStatement          = "SELECT FROM pg_database WHERE datname = ?"
	postgresqlCreateDBStatement         = "CREATE DATABASE %s"
	postgresqlUserExists                = "SELECT 1 FROM pg_roles WHERE rolname = ?"
	postgresqlCreateUserStatement       = "CREATE ROLE %s LOGIN PASSWORD ?"
	postgresqlShowGrantsStatement       = "SELECT has_database_privilege(rolname, ?, 'create') from pg_roles where rolcanlogin and rolname = ?"
	postgresqlGrantPrivilegesStatement  = "GRANT ALL PRIVILEGES ON DATABASE %s TO %s"
	postgresqlRevokePrivilegesStatement = "REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s"
	postgresqlDropRoleStatement         = "DROP ROLE %s"
	postgresqlDropDBStatement           = "DROP DATABASE %s WITH (FORCE)"
)

type PostgreSQLConnection struct {
	db   *pg.DB
	host string
	port int
}

func (r *PostgreSQLConnection) Driver() string {
	return "PostgreSQL"
}

func (r *PostgreSQLConnection) UserExists(ctx context.Context, user string) (bool, error) {
	res, err := r.db.ExecContext(ctx, postgresqlUserExists, user)
	if err != nil {
		return false, err
	}

	return res.RowsReturned() > 0, nil
}

func (r *PostgreSQLConnection) CreateUser(ctx context.Context, user, password string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlCreateUserStatement, user), password)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgreSQLConnection) DBExists(ctx context.Context, dbName string) (bool, error) {
	rows, err := r.db.ExecContext(ctx, postgresqlFetchDBStatement, dbName)
	if err != nil {
		return false, err
	}

	return rows.RowsReturned() > 0, nil
}

func (r *PostgreSQLConnection) CreateDB(ctx context.Context, dbName string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlCreateDBStatement, dbName))
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgreSQLConnection) GrantPrivilegesExists(ctx context.Context, user, dbName string) (bool, error) {
	var hasDatabasePrivilege string

	_, err := r.db.QueryContext(ctx, pg.Scan(&hasDatabasePrivilege), postgresqlShowGrantsStatement, dbName, user)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return false, nil
		}

		return false, err
	}

	return hasDatabasePrivilege == "t", nil
}

func (r *PostgreSQLConnection) GrantPrivileges(ctx context.Context, user, dbName string) error {
	res, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlGrantPrivilegesStatement, dbName, user))
	_ = res

	return err
}

func (r *PostgreSQLConnection) DeleteUser(ctx context.Context, user string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlDropRoleStatement, user))

	return err
}

func (r *PostgreSQLConnection) DeleteDB(ctx context.Context, dbName string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlDropDBStatement, dbName))

	return err
}

func (r *PostgreSQLConnection) RevokePrivileges(ctx context.Context, user, dbName string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlRevokePrivilegesStatement, dbName, user))

	return err
}

func (r *PostgreSQLConnection) GetHost() string {
	return r.host
}

func (r *PostgreSQLConnection) GetPort() int {
	return r.port
}

func (r *PostgreSQLConnection) Close() error {
	return r.db.Close()
}

func (r *PostgreSQLConnection) Check() error {
	return r.db.Ping(context.Background())
}
