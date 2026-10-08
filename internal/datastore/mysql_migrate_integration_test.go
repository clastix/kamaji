// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// TestMySQLMigrateHexBlobRoundTrip is a regression test for the data-loss bug in the
// previous go-mysqldump-based Migrate(): that library wrote BLOB/TEXT values with zero
// escaping, so a value containing a backslash immediately followed by a quote silently lost
// bytes on restore - exactly the shape of Kubernetes-stored objects (managedFields,
// escaped-JSON annotations). The current Migrate() moves values as bind parameters, never
// interpolated into SQL text, so this is no longer a class of bug that can occur; this test
// pins that down with the exact byte pattern that used to trigger it.
//
// Opt-in only (needs Docker): KAMAJI_MYSQL_MIGRATE_IT=1 go test ./internal/datastore/ -run TestMySQLMigrate -v.
func TestMySQLMigrateHexBlobRoundTrip(t *testing.T) {
	if os.Getenv("KAMAJI_MYSQL_MIGRATE_IT") == "" {
		t.Skip("set KAMAJI_MYSQL_MIGRATE_IT=1 to run (spins up real MariaDB containers via Docker)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	source := startMariaDB(ctx, t)
	target := startMariaDB(ctx, t)

	const schema = "testschema"

	original := []byte("prefix\\'managedFields-like-payload\\'suffix\x00\xffbinary")

	setupDB := openMariaDB(t, source, "")
	defer setupDB.Close()

	mustExec(ctx, t, setupDB, fmt.Sprintf("CREATE DATABASE `%s`", schema))
	mustExec(ctx, t, setupDB, kineTableDDL(schema))

	if _, err := setupDB.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s`.kine (id, name, value) VALUES (1, '/registry/test', ?)", schema), original); err != nil {
		t.Fatalf("seed source row: %v", err)
	}

	sourceConn := mustMySQLConnection(t, source)
	targetConn := mustMySQLConnection(t, target)

	tcp := kamajiv1alpha1.TenantControlPlane{ObjectMeta: metav1.ObjectMeta{UID: types.UID("test-migrate-uid")}}
	tcp.Status.Storage.Setup.Schema = schema

	if err := sourceConn.Migrate(ctx, tcp, targetConn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	verifyDB := openMariaDB(t, target, schema)
	defer verifyDB.Close()

	var got []byte

	if err := verifyDB.QueryRowContext(ctx, "SELECT value FROM kine WHERE id = 1").Scan(&got); err != nil {
		t.Fatalf("read back migrated row: %v", err)
	}

	if !bytes.Equal(got, original) {
		t.Fatalf("migrated value corrupted:\n got  = %q (%d bytes)\n want = %q (%d bytes)", got, len(got), original, len(original))
	}
}

// TestMySQLMigrateNullValues checks that NULL columns (old_value is NULL on a fresh key,
// name/lease/etc. can be NULL too) round-trip as NULL, not as a zero-length/placeholder value.
func TestMySQLMigrateNullValues(t *testing.T) {
	if os.Getenv("KAMAJI_MYSQL_MIGRATE_IT") == "" {
		t.Skip("set KAMAJI_MYSQL_MIGRATE_IT=1 to run (spins up real MariaDB containers via Docker)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	source := startMariaDB(ctx, t)
	target := startMariaDB(ctx, t)

	const schema = "testschema"

	setupDB := openMariaDB(t, source, "")
	defer setupDB.Close()

	mustExec(ctx, t, setupDB, fmt.Sprintf("CREATE DATABASE `%s`", schema))
	mustExec(ctx, t, setupDB, kineTableDDL(schema))

	if _, err := setupDB.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s`.kine (id, name, lease, value, old_value) VALUES (1, ?, ?, ?, ?)", schema), nil, nil, nil, nil); err != nil {
		t.Fatalf("seed source row: %v", err)
	}

	sourceConn := mustMySQLConnection(t, source)
	targetConn := mustMySQLConnection(t, target)

	tcp := kamajiv1alpha1.TenantControlPlane{ObjectMeta: metav1.ObjectMeta{UID: types.UID("test-migrate-uid")}}
	tcp.Status.Storage.Setup.Schema = schema

	if err := sourceConn.Migrate(ctx, tcp, targetConn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	verifyDB := openMariaDB(t, target, schema)
	defer verifyDB.Close()

	var name, lease, value, oldValue sql.NullString

	if err := verifyDB.QueryRowContext(ctx, "SELECT name, lease, value, old_value FROM kine WHERE id = 1").Scan(&name, &lease, &value, &oldValue); err != nil {
		t.Fatalf("read back migrated row: %v", err)
	}

	for colName, col := range map[string]sql.NullString{"name": name, "lease": lease, "value": value, "old_value": oldValue} {
		if col.Valid {
			t.Errorf("column %s: expected NULL, got %q", colName, col.String)
		}
	}
}

// TestMySQLMigrateBatching exercises more rows than migrateBatchSize, so the migration
// spans multiple INSERT batches, and checks every row survives in order.
func TestMySQLMigrateBatching(t *testing.T) {
	if os.Getenv("KAMAJI_MYSQL_MIGRATE_IT") == "" {
		t.Skip("set KAMAJI_MYSQL_MIGRATE_IT=1 to run (spins up real MariaDB containers via Docker)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	source := startMariaDB(ctx, t)
	target := startMariaDB(ctx, t)

	const schema = "testschema"

	rowCount := migrateBatchSize*2 + 7

	setupDB := openMariaDB(t, source, "")
	defer setupDB.Close()

	mustExec(ctx, t, setupDB, fmt.Sprintf("CREATE DATABASE `%s`", schema))
	mustExec(ctx, t, setupDB, kineTableDDL(schema))

	for i := 1; i <= rowCount; i++ {
		if _, err := setupDB.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s`.kine (id, name, value) VALUES (?, ?, ?)", schema), i, fmt.Sprintf("/registry/test/%d", i), []byte(fmt.Sprintf("value-%d", i))); err != nil {
			t.Fatalf("seed source row %d: %v", i, err)
		}
	}

	sourceConn := mustMySQLConnection(t, source)
	targetConn := mustMySQLConnection(t, target)

	tcp := kamajiv1alpha1.TenantControlPlane{ObjectMeta: metav1.ObjectMeta{UID: types.UID("test-migrate-uid")}}
	tcp.Status.Storage.Setup.Schema = schema

	if err := sourceConn.Migrate(ctx, tcp, targetConn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	verifyDB := openMariaDB(t, target, schema)
	defer verifyDB.Close()

	var count int
	if err := verifyDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM kine").Scan(&count); err != nil {
		t.Fatalf("count migrated rows: %v", err)
	}

	if count != rowCount {
		t.Fatalf("expected %d migrated rows, got %d", rowCount, count)
	}

	var value []byte
	if err := verifyDB.QueryRowContext(ctx, "SELECT value FROM kine WHERE id = ?", rowCount).Scan(&value); err != nil {
		t.Fatalf("read back last row: %v", err)
	}

	if want := fmt.Sprintf("value-%d", rowCount); string(value) != want {
		t.Fatalf("last row value = %q, want %q", value, want)
	}
}

// TestMySQLMigrateDiscoversTables checks that Migrate() doesn't assume the schema contains
// only (or even) a table named "kine": it should copy whatever base tables are actually
// there, each with its own column set, the same thing a schema-wide dump would do.
func TestMySQLMigrateDiscoversTables(t *testing.T) {
	if os.Getenv("KAMAJI_MYSQL_MIGRATE_IT") == "" {
		t.Skip("set KAMAJI_MYSQL_MIGRATE_IT=1 to run (spins up real MariaDB containers via Docker)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	source := startMariaDB(ctx, t)
	target := startMariaDB(ctx, t)

	const schema = "testschema"

	setupDB := openMariaDB(t, source, "")
	defer setupDB.Close()

	mustExec(ctx, t, setupDB, fmt.Sprintf("CREATE DATABASE `%s`", schema))
	mustExec(ctx, t, setupDB, fmt.Sprintf("CREATE TABLE `%s`.widgets (widget_id INT PRIMARY KEY, label VARCHAR(100), payload VARBINARY(64))", schema))
	mustExec(ctx, t, setupDB, fmt.Sprintf("CREATE TABLE `%s`.gadgets (gadget_uuid CHAR(36) PRIMARY KEY, weight DOUBLE)", schema))

	if _, err := setupDB.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s`.widgets (widget_id, label, payload) VALUES (1, 'first', ?)", schema), []byte("raw\x00bytes")); err != nil {
		t.Fatalf("seed widgets row: %v", err)
	}

	if _, err := setupDB.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s`.gadgets (gadget_uuid, weight) VALUES ('11111111-1111-1111-1111-111111111111', 3.5)", schema)); err != nil {
		t.Fatalf("seed gadgets row: %v", err)
	}

	sourceConn := mustMySQLConnection(t, source)
	targetConn := mustMySQLConnection(t, target)

	tcp := kamajiv1alpha1.TenantControlPlane{ObjectMeta: metav1.ObjectMeta{UID: types.UID("test-migrate-uid")}}
	tcp.Status.Storage.Setup.Schema = schema

	if err := sourceConn.Migrate(ctx, tcp, targetConn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	verifyDB := openMariaDB(t, target, schema)
	defer verifyDB.Close()

	var label string

	var payload []byte

	if err := verifyDB.QueryRowContext(ctx, "SELECT label, payload FROM widgets WHERE widget_id = 1").Scan(&label, &payload); err != nil {
		t.Fatalf("read back widgets row: %v", err)
	}

	if label != "first" || !bytes.Equal(payload, []byte("raw\x00bytes")) {
		t.Fatalf("widgets row mismatch: label=%q payload=%q", label, payload)
	}

	var weight float64

	if err := verifyDB.QueryRowContext(ctx, "SELECT weight FROM gadgets WHERE gadget_uuid = '11111111-1111-1111-1111-111111111111'").Scan(&weight); err != nil {
		t.Fatalf("read back gadgets row: %v", err)
	}

	if weight != 3.5 { //nolint:testifylint
		t.Fatalf("gadgets row weight = %v, want 3.5", weight)
	}
}

// kineTableDDL returns a CREATE TABLE statement for a schema-qualified kine table shaped like
// a real kine sidecar's (fewer indexes, Migrate() doesn't care about those).
func kineTableDDL(schema string) string {
	return fmt.Sprintf(`CREATE TABLE `+"`%s`"+`.kine (
		id BIGINT UNSIGNED PRIMARY KEY,
		name VARCHAR(630),
		created INT,
		deleted INT,
		create_revision BIGINT UNSIGNED,
		prev_revision BIGINT UNSIGNED,
		lease INT,
		value MEDIUMBLOB,
		old_value MEDIUMBLOB
	)`, schema)
}

func mustExec(ctx context.Context, t *testing.T, db *sql.DB, query string) {
	t.Helper()

	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func mustMySQLConnection(t *testing.T, c mariadbContainer) Connection {
	t.Helper()

	conn, err := NewMySQLConnection(ConnectionConfig{
		User:      "root",
		Password:  "root",
		Endpoints: []ConnectionEndpoint{{Host: c.host, Port: c.port}},
	})
	if err != nil {
		t.Fatalf("NewMySQLConnection: %v", err)
	}

	return conn
}

type mariadbContainer struct {
	testcontainers.Container
	host string
	port int
}

func openMariaDB(t *testing.T, c mariadbContainer, dbName string) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", fmt.Sprintf("root:root@tcp(%s:%d)/%s?parseTime=true", c.host, c.port, dbName))
	if err != nil {
		t.Fatalf("open mariadb connection: %v", err)
	}

	return db
}

func startMariaDB(ctx context.Context, t *testing.T) mariadbContainer {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "mariadb:11.4",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MARIADB_ROOT_PASSWORD": "root",
		},
		WaitingFor: wait.ForLog("ready for connections").WithOccurrence(2).WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start mariadb container: %v", err)
	}

	t.Cleanup(func() {
		// ctx is cancelled by this point (the test's defer cancel() already ran), so
		// WithoutCancel carries it forward without letting that abort the termination call.
		_ = container.Terminate(context.WithoutCancel(ctx))
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		t.Fatalf("container mapped port: %v", err)
	}

	return mariadbContainer{Container: container, host: host, port: int(mappedPort.Num())}
}
