// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-pg/pg/v10"
	goerrors "github.com/pkg/errors"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/datastore/errors"
)

const (
	postgresqlFetchDBStatement            = "SELECT FROM pg_database WHERE datname = ?"
	postgresqlCreateDBStatement           = "CREATE DATABASE %s"
	postgresqlUserExists                  = "SELECT 1 FROM pg_roles WHERE rolname = ?"
	postgresqlCreateUserStatement         = "CREATE ROLE %s LOGIN PASSWORD ?"
	postgresqlShowGrantsStatement         = "SELECT has_database_privilege(rolname, ?, 'create') from pg_roles where rolcanlogin and rolname = ?"
	postgresqlShowOwnershipStatement      = "SELECT 't' FROM pg_catalog.pg_database AS d WHERE d.datname = ? AND pg_catalog.pg_get_userbyid(d.datdba) = ?"
	postgresqlShowTableOwnershipStatement = "SELECT 't' from pg_tables where tableowner = ? AND tablename = ?"
	postgresqlKineTableExistsStatement    = "SELECT 't' FROM pg_tables WHERE schemaname = ? AND tablename  = ?"
	postgresqlGrantPrivilegesStatement    = "GRANT CONNECT, CREATE ON DATABASE %s TO %s"
	postgresqlChangeOwnerStatement        = "ALTER DATABASE %s OWNER TO %s"
	postgresqlRevokePrivilegesStatement   = "REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s"
	postgresqlDropRoleStatement           = "DROP ROLE %s"
	postgresqlDropDBStatement             = "DROP DATABASE %s WITH (FORCE)"
)

type PostgreSQLConnection struct {
	db               *pg.DB
	connection       ConnectionEndpoint
	rootUser         string
	switchDatabaseFn func(dbName string) *pg.DB
}

func (r *PostgreSQLConnection) Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error {
	// Ensuring the connection is working as expected
	if err := target.Check(ctx); err != nil {
		return fmt.Errorf("unable to check target datastore: %w", err)
	}
	// Creating the target schema if it doesn't exist
	if ok, _ := target.DBExists(ctx, tcp.Status.Storage.Setup.Schema); !ok {
		if err := target.CreateDB(ctx, tcp.Status.Storage.Setup.Schema); err != nil {
			return err
		}
	}

	targetConn := target.(*PostgreSQLConnection).switchDatabaseFn(tcp.Status.Storage.Setup.Schema) //nolint:forcetypeassert

	err := targetConn.RunInTransaction(ctx, func(tx *pg.Tx) error {
		for _, stm := range []string{
			`CREATE TABLE IF NOT EXISTS kine (
				id SERIAL PRIMARY KEY,
				name VARCHAR(630),
				created INTEGER,
				deleted INTEGER,
				create_revision INTEGER,
				prev_revision INTEGER,
				lease INTEGER,
				value bytea,
				old_value bytea
			)`,
			`TRUNCATE TABLE kine`,
			`CREATE INDEX IF NOT EXISTS kine_name_index ON kine (name)`,
			`CREATE INDEX IF NOT EXISTS kine_name_id_index ON kine (name,id)`,
			`CREATE INDEX IF NOT EXISTS kine_id_deleted_index ON kine (id,deleted)`,
			`CREATE INDEX IF NOT EXISTS kine_prev_revision_index ON kine (prev_revision)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS kine_name_prev_revision_uindex ON kine (name, prev_revision)`,
		} {
			if _, err := tx.ExecContext(ctx, stm); err != nil {
				return fmt.Errorf("unable to perform schema creation: %w", err)
			}
		}
		// Dumping the old datastore in a local buffer
		var buf bytes.Buffer

		if _, err := r.switchDatabaseFn(tcp.Status.Storage.Setup.Schema).WithContext(ctx).CopyTo(&buf, "COPY kine TO STDOUT"); err != nil {
			return fmt.Errorf("unable to copy from the origin datastore: %w", err)
		}

		if _, err := tx.CopyFrom(&buf, "COPY kine FROM STDIN"); err != nil {
			return fmt.Errorf("unable to copy to the target datastore: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to perform migration transaction: %w", err)
	}

	return nil
}

func NewPostgreSQLConnection(config ConnectionConfig) (Connection, error) {
	opt := &pg.Options{
		Addr:      config.Endpoints[0].String(),
		Database:  config.DBName,
		User:      config.User,
		Password:  config.Password,
		TLSConfig: config.TLSConfig,
	}

	fn := func(dbName string) *pg.DB {
		o := opt
		o.Database = dbName

		return pg.Connect(o)
	}

	return &PostgreSQLConnection{
		db:               pg.Connect(opt),
		switchDatabaseFn: fn,
		rootUser:         config.User,
		connection:       config.Endpoints[0],
	}, nil
}

func (r *PostgreSQLConnection) Driver() string {
	return string(kamajiv1alpha1.KinePostgreSQLDriver)
}

func (r *PostgreSQLConnection) UserExists(ctx context.Context, user string) (bool, error) {
	res, err := r.db.ExecContext(ctx, postgresqlUserExists, user)
	if err != nil {
		return false, errors.NewCheckUserExistsError(err)
	}

	return res.RowsReturned() > 0, nil
}

func (r *PostgreSQLConnection) CreateUser(ctx context.Context, user, password string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlCreateUserStatement, user), password)
	if err != nil {
		return errors.NewCreateUserError(err)
	}

	return nil
}

func (r *PostgreSQLConnection) DBExists(ctx context.Context, dbName string) (bool, error) {
	rows, err := r.db.ExecContext(ctx, postgresqlFetchDBStatement, dbName)
	if err != nil {
		return false, errors.NewCheckDatabaseExistError(err)
	}

	return rows.RowsReturned() > 0, nil
}

func (r *PostgreSQLConnection) CreateDB(ctx context.Context, dbName string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlCreateDBStatement, dbName))
	if err != nil {
		return errors.NewCreateDBError(err)
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

		return false, errors.NewCheckGrantExistsError(err)
	}

	var isOwner string

	if _, err = r.db.QueryContext(ctx, pg.Scan(&isOwner), postgresqlShowOwnershipStatement, dbName, user); err != nil {
		return false, errors.NewCheckGrantExistsError(err)
	}

	var isTableOwner string

	dbConn := r.switchDatabaseFn(dbName)
	defer dbConn.Close()

	tableExists, err := r.kineTableExists(ctx, dbConn)
	if err != nil {
		return false, errors.NewGrantPrivilegesError(err)
	}

	if tableExists {
		if _, err = dbConn.QueryContext(ctx, pg.Scan(&isTableOwner), postgresqlShowTableOwnershipStatement, user, "kine"); err != nil {
			return false, errors.NewCheckGrantExistsError(err)
		}

		return hasDatabasePrivilege == "t" && isOwner == "t" && isTableOwner == "t", nil
	}

	return hasDatabasePrivilege == "t" && isOwner == "t", nil
}

func (r *PostgreSQLConnection) GrantPrivileges(ctx context.Context, user, dbName string) error {
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlGrantPrivilegesStatement, dbName, user)); err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	dbConn := r.switchDatabaseFn(dbName)
	defer dbConn.Close()

	if _, err := dbConn.ExecContext(ctx, fmt.Sprintf(postgresqlChangeOwnerStatement, dbName, user)); err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	tableExists, err := r.kineTableExists(ctx, dbConn)
	if err != nil {
		return errors.NewGrantPrivilegesError(err)
	}

	if tableExists {
		if _, err = dbConn.ExecContext(ctx, fmt.Sprintf("ALTER TABLE kine OWNER TO %s", user)); err != nil {
			return errors.NewGrantPrivilegesError(err)
		}
	}

	return nil
}

func (r *PostgreSQLConnection) DeleteUser(ctx context.Context, user string) error {
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlDropRoleStatement, user)); err != nil {
		return errors.NewDeleteUserError(err)
	}

	return nil
}

func (r *PostgreSQLConnection) DeleteDB(ctx context.Context, dbName string) error {
	if err := r.GrantPrivileges(ctx, r.rootUser, dbName); err != nil {
		return errors.NewCannotDeleteDatabaseError(goerrors.Wrap(err, "cannot grant privileges to root user"))
	}

	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlDropDBStatement, dbName)); err != nil {
		return errors.NewCannotDeleteDatabaseError(err)
	}

	return nil
}

func (r *PostgreSQLConnection) RevokePrivileges(ctx context.Context, user, dbName string) error {
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(postgresqlRevokePrivilegesStatement, dbName, user)); err != nil {
		return errors.NewRevokePrivilegesError(err)
	}

	return nil
}

func (r *PostgreSQLConnection) GetConnectionString() string {
	return r.connection.String()
}

func (r *PostgreSQLConnection) Close() error {
	if err := r.db.Close(); err != nil {
		return errors.NewCloseConnectionError(err)
	}

	return nil
}

func (r *PostgreSQLConnection) Check(ctx context.Context) error {
	if err := r.db.Ping(ctx); err != nil {
		return errors.NewCheckConnectionError(err)
	}

	return nil
}

func (r *PostgreSQLConnection) kineTableExists(ctx context.Context, db *pg.DB) (bool, error) {
	var tableExists string

	if _, err := db.QueryContext(ctx, pg.Scan(&tableExists), postgresqlKineTableExistsStatement, "public", "kine"); err != nil {
		return false, err
	}

	return tableExists == "t", nil
}
