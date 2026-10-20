// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"testing"
)

func TestPostgreSQLSwitchDatabaseKeepsRootOptions(t *testing.T) {
	conn, err := NewPostgreSQLConnection(ConnectionConfig{
		User:      "postgres",
		Endpoints: []ConnectionEndpoint{{Host: "127.0.0.1", Port: 5432}},
		DBName:    "postgres",
	})
	if err != nil {
		t.Fatalf("NewPostgreSQLConnection() error = %v", err)
	}

	root, ok := conn.(*PostgreSQLConnection)
	if !ok {
		t.Fatalf("NewPostgreSQLConnection() returned %T, want *PostgreSQLConnection", conn)
	}
	t.Cleanup(func() { _ = root.Close() })

	tenant := root.switchDatabaseFn("tenant")
	t.Cleanup(func() { _ = tenant.Close() })

	if got := tenant.Options().Database; got != "tenant" {
		t.Fatalf("switched connection database = %q, want %q", got, "tenant")
	}
	if got := root.db.Options().Database; got != "postgres" {
		t.Fatalf("root connection database = %q after switching, want %q", got, "postgres")
	}
}
