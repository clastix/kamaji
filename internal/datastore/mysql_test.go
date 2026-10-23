// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"crypto/tls"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestMySQLConnectionDoesNotRegisterGlobalTLSConfig(t *testing.T) {
	conn, err := NewMySQLConnection(ConnectionConfig{
		User:      "root",
		Endpoints: []ConnectionEndpoint{{Host: "127.0.0.1", Port: 3306}},
		DBName:    "kine",
		TLSConfig: &tls.Config{ServerName: "datastore-a", MinVersion: tls.VersionTLS12},
	})
	if err != nil {
		t.Fatalf("NewMySQLConnection() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, err := mysql.ParseDSN("root@tcp(127.0.0.1:3306)/kine?tls=mysql"); err == nil {
		t.Fatal(`TLS config is registered under the process-global key "mysql", want it kept on the connection`)
	}
}
