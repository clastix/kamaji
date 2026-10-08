// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-sql-driver/mysql"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/datastore/errors"
)

// migrateBatchSize caps how many rows are sent per INSERT during a migration. Kept modest
// since a single value can be large (kine's value/old_value are mediumblob, up to 16MB) and a
// batch shouldn't risk tripping the target server's max_allowed_packet.
const migrateBatchSize = 100

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

// Migrate copies every base table in the schema from c (origin) to target, matching the
// approach already used by PostgreSQLConnection.Migrate() and EtcdClient.Migrate(): read and
// write directly over the native Go client, in-process, rather than shelling out to or
// reimplementing a text-based dump tool. Row values move as bind parameters end to end and
// are never interpolated into SQL text, so there's no string-escaping step where a byte could
// be lost. Tables are discovered rather than assumed to be just kine's: a kine-backed schema
// only ever has the one table in practice, but this mirrors what the schema actually contains
// instead of hard-coding that.
func (c *MySQLConnection) Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error {
	if err := target.Check(ctx); err != nil {
		return err
	}

	schema := tcp.Status.Storage.Setup.Schema

	if ok, _ := target.DBExists(ctx, schema); !ok {
		if err := target.CreateDB(ctx, schema); err != nil {
			return err
		}
	}

	targetClient := target.(*MySQLConnection) //nolint:forcetypeassert

	// A consistent snapshot at the start of this read-only transaction, same guarantee
	// mysqldump's --single-transaction gives: concurrent writes on the source don't leak
	// partial state into what gets copied, across every table. BeginTx pins its own
	// connection for as long as the transaction is open, so nothing else needs to.
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return fmt.Errorf("unable to start consistent-snapshot transaction for MySQL migration: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	tableRows, err := tx.QueryContext(ctx, "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME", schema)
	if err != nil {
		return fmt.Errorf("unable to list tables for MySQL migration: %w", err)
	}
	defer tableRows.Close() //nolint:errcheck

	var tables []string

	for tableRows.Next() {
		var table string
		if err = tableRows.Scan(&table); err != nil {
			return fmt.Errorf("unable to read table name for MySQL migration: %w", err)
		}

		tables = append(tables, table)
	}

	if err = tableRows.Err(); err != nil {
		return fmt.Errorf("error listing tables for MySQL migration: %w", err)
	}

	for _, table := range tables {
		if err = migrateTable(ctx, tx, targetClient.db, schema, table); err != nil {
			return fmt.Errorf("unable to migrate table %q for MySQL migration: %w", table, err)
		}
	}

	return nil
}

// migrateTable copies a single table's DDL and rows from the snapshot transaction tx to
// targetDB, discovering columns rather than assuming a fixed shape.
func migrateTable(ctx context.Context, tx *sql.Tx, targetDB *sql.DB, schema, table string) error {
	qualifiedTable := quoteMySQLIdentifier(schema) + "." + quoteMySQLIdentifier(table)

	// Replay the table's exact DDL from the source rather than hard-coding it, so this
	// doesn't need to track its schema independently: whatever columns/indexes it actually
	// has is what gets recreated on the target. SHOW CREATE TABLE's output always starts
	// with CREATE TABLE `<table>` (<table> unqualified even when queried qualified), so that
	// prefix is rewritten to target the right schema.
	var name, createTableStatement string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE %s", qualifiedTable)).Scan(&name, &createTableStatement); err != nil {
		return fmt.Errorf("unable to read table definition: %w", err)
	}

	createTableStatement = strings.Replace(createTableStatement, "CREATE TABLE "+quoteMySQLIdentifier(table), "CREATE TABLE "+qualifiedTable, 1)

	if _, err := targetDB.ExecContext(ctx, createTableStatement); err != nil {
		return fmt.Errorf("unable to create table on target: %w", err)
	}

	// Deliberately not listing columns here: this table's shape was just discovered from
	// SHOW CREATE TABLE above, not assumed, so there's no fixed column list to name.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", qualifiedTable)) //nolint:unqueryvet
	if err != nil {
		return fmt.Errorf("unable to read rows: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("unable to read columns: %w", err)
	}

	batch := make([][]any, 0, migrateBatchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		if err := insertBatch(ctx, targetDB, qualifiedTable, columns, batch); err != nil {
			return err
		}

		batch = batch[:0]

		return nil
	}

	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))

		for i := range values {
			pointers[i] = &values[i]
		}

		if err = rows.Scan(pointers...); err != nil {
			return fmt.Errorf("unable to scan row: %w", err)
		}

		batch = append(batch, values)

		if len(batch) == migrateBatchSize {
			if err = flush(); err != nil {
				return fmt.Errorf("unable to write batch: %w", err)
			}
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	return flush()
}

// insertBatch writes rows to the target table in a single multi-row INSERT, with every value
// passed as a bind parameter rather than interpolated into the statement text.
func insertBatch(ctx context.Context, db *sql.DB, qualifiedTable string, columns []string, rows [][]any) error {
	quotedColumns := make([]string, len(columns))
	for i, column := range columns {
		quotedColumns[i] = quoteMySQLIdentifier(column)
	}

	placeholders := "(" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"

	var sb strings.Builder

	sb.WriteString("INSERT INTO " + qualifiedTable + " (" + strings.Join(quotedColumns, ",") + ") VALUES ")

	args := make([]any, 0, len(rows)*len(columns))

	for i, row := range rows {
		if i > 0 {
			sb.WriteString(",")
		}

		sb.WriteString(placeholders)

		args = append(args, row...)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)

	return err
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
	statement, err := c.db.PrepareContext(ctx, nonFilledStatement)
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
