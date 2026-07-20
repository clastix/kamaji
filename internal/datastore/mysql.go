// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
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
	// Identifiers (database and user names) cannot be passed as bind parameters,
	// so the `%s` verbs below must only ever be fed values run through
	// quoteMySQLIdentifier; the password literal must be fed escapeMySQLString.
	mysqlFetchUserStatement        = "SELECT User FROM mysql.user WHERE User= ? LIMIT 1"
	mysqlFetchDBStatement          = "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME=? LIMIT 1"
	mysqlCreateDBStatement         = "CREATE DATABASE IF NOT EXISTS %s"
	mysqlCreateUserStatement       = "CREATE USER %s@`%%` IDENTIFIED BY '%s'"
	mysqlUpdateUserStatement       = "ALTER USER %s@`%%` IDENTIFIED BY '%s'"
	mysqlGrantPrivilegesStatement  = "GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX ON %s.* TO %s@`%%`"
	mysqlDropDBStatement           = "DROP DATABASE IF EXISTS %s"
	mysqlDropUserStatement         = "DROP USER IF EXISTS %s"
	mysqlRevokePrivilegesStatement = "REVOKE ALL PRIVILEGES ON %s.* FROM %s"
	mysqlCheckGrantsStatement      = `
		SELECT 1
		FROM mysql.db
		WHERE user = ? AND db = ? AND host = '%'
		  AND Select_priv = 'Y'
		  AND Insert_priv = 'Y'
		  AND Update_priv = 'Y'
		  AND Delete_priv = 'Y'
		  AND Create_priv = 'Y'
		  AND Alter_priv = 'Y'
		  AND Index_priv = 'Y'
	`
)

type MySQLConnection struct {
	db        *sql.DB
	config    *mysql.Config
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

	if _, err = c.db.ExecContext(ctx, fmt.Sprintf("USE %s", quoteMySQLIdentifier(tcp.Status.Storage.Setup.Schema))); err != nil {
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

	// The dump is a batch of semicolon-separated statements, so it must run over
	// a connection with multiStatements enabled. That connection is scoped to
	// this import alone and closed right after, keeping the primary connection
	// single-statement.
	importDB, err := targetClient.multiStatementConn()
	if err != nil {
		return fmt.Errorf("unable to open MySQL multi-statement connection for migration: %w", err)
	}
	defer importDB.Close()

	if _, err = importDB.ExecContext(ctx, fmt.Sprintf("USE %s", quoteMySQLIdentifier(tcp.Status.Storage.Setup.Schema))); err != nil {
		return fmt.Errorf("unable to switch DB for MySQL migration: %w", err)
	}

	if _, err = importDB.ExecContext(ctx, string(statements)); err != nil {
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

	return &MySQLConnection{db: db, config: mysqlConfig, connector: config.Endpoints[0]}, nil
}

// multiStatementConn opens a dedicated connection whose DSN carries the
// multiStatements driver parameter, required to execute a mysqldump batch
// (many semicolon-separated statements) in a single ExecContext call. It is
// deliberately kept out of the primary connection so that statements
// interpolating tenant-controlled identifiers never run over a connection that
// permits stacked queries. The pool is capped at a single connection so the
// USE statement and the subsequent import share the same session.
func (c *MySQLConnection) multiStatementConn() (*sql.DB, error) {
	cfg := c.config.Clone()
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["multiStatements"] = "true"

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	return db, nil
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
	if err := c.mutate(ctx, mysqlCreateUserStatement, quoteMySQLIdentifier(user), escapeMySQLString(password)); err != nil {
		return errors.NewCreateUserError(err)
	}

	return nil
}

func (c *MySQLConnection) UpdateUser(ctx context.Context, user, password string) error {
	if err := c.mutate(ctx, mysqlUpdateUserStatement, quoteMySQLIdentifier(user), escapeMySQLString(password)); err != nil {
		return errors.NewUpdateUserError(err)
	}

	return nil
}

func (c *MySQLConnection) CreateDB(ctx context.Context, dbName string) error {
	if err := c.mutate(ctx, mysqlCreateDBStatement, quoteMySQLIdentifier(dbName)); err != nil {
		return errors.NewCreateDBError(err)
	}

	return nil
}

func (c *MySQLConnection) GrantPrivileges(ctx context.Context, user, dbName string) error {
	if err := c.mutate(ctx, mysqlGrantPrivilegesStatement, quoteMySQLIdentifier(dbName), quoteMySQLIdentifier(user)); err != nil {
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

func (c *MySQLConnection) GrantPrivilegesExists(ctx context.Context, user, dbName string) (bool, error) {
	var exists int

	if err := c.db.QueryRowContext(ctx, mysqlCheckGrantsStatement, user, dbName).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, errors.NewCheckGrantExistsError(err)
	}

	return true, nil
}

func (c *MySQLConnection) DeleteUser(ctx context.Context, user string) error {
	if err := c.mutate(ctx, mysqlDropUserStatement, quoteMySQLIdentifier(user)); err != nil {
		return errors.NewDeleteUserError(err)
	}

	return nil
}

func (c *MySQLConnection) DeleteDB(ctx context.Context, dbName string) error {
	if err := c.mutate(ctx, mysqlDropDBStatement, quoteMySQLIdentifier(dbName)); err != nil {
		return errors.NewCannotDeleteDatabaseError(err)
	}

	return nil
}

func (c *MySQLConnection) RevokePrivileges(ctx context.Context, user, dbName string) error {
	if err := c.mutate(ctx, mysqlRevokePrivilegesStatement, quoteMySQLIdentifier(dbName), quoteMySQLIdentifier(user)); err != nil {
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

// quoteMySQLIdentifier safely quotes a MySQL identifier (such as a database or
// user name) so it can be interpolated into a statement: it wraps the value in
// backticks and doubles any embedded backtick, neutralising attempts to break
// out of the identifier. NUL bytes, which are illegal in identifiers, are
// stripped. Identifiers cannot be supplied as bind parameters, hence the manual
// quoting.
func quoteMySQLIdentifier(identifier string) string {
	identifier = strings.ReplaceAll(identifier, "\x00", "")

	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

// escapeMySQLString escapes a value for safe embedding inside a single-quoted
// MySQL string literal. Both ways of breaking out of such a literal are closed
// in a manner that holds under every sql_mode: single quotes are doubled (” is
// a literal quote regardless of NO_BACKSLASH_ESCAPES) and backslashes are
// doubled (so a trailing backslash cannot escape the closing quote when
// backslash escaping is enabled). The remaining control-character escapes are
// conveniences for the default sql_mode. DDL statements such as CREATE USER
// cannot bind the password as a parameter, hence the manual escaping.
func escapeMySQLString(value string) string {
	var b strings.Builder

	for _, r := range value {
		switch r {
		case 0:
			b.WriteString(`\0`)
		case '\'':
			b.WriteString(`''`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case 26: // Ctrl+Z
			b.WriteString(`\Z`)
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}
