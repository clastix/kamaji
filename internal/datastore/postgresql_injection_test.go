// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"fmt"
	"strings"
	"testing"
)

// TestQuotePostgreSQLIdentifierNeutralisesInjection ensures a malicious
// identifier cannot break out of the double-quote quoting and stack additional
// statements, which is the regression behind the dataStoreSchema/
// dataStoreUsername SQL injection.
func TestQuotePostgreSQLIdentifierNeutralisesInjection(t *testing.T) {
	// A tenant-controlled value attempting to stack a DROP DATABASE statement by
	// closing the identifier with a double quote (PostgreSQL executes stacked
	// statements over the simple query protocol used for these DDL calls).
	payload := `x"; DROP DATABASE victim; -- `

	// Every embedded double quote is doubled, so the whole payload stays a single
	// quoted identifier instead of terminating it early.
	want := `"x""; DROP DATABASE victim; -- "`
	if got := quotePostgreSQLIdentifier(payload); got != want {
		t.Fatalf("quotePostgreSQLIdentifier(%q) = %q, want %q", payload, got, want)
	}

	// Structural safety property: stripping the outer quotes and collapsing the
	// doubled quotes must leave no lone double quote that could close the
	// identifier prematurely.
	inner := want[1 : len(want)-1]
	if strings.Contains(strings.ReplaceAll(inner, `""`, ""), `"`) {
		t.Fatalf("a lone double quote survived quoting: %q", want)
	}
}

// TestQuotePostgreSQLIdentifierPreservesDottedNames guards the defaulter, which
// can produce dotted database/role names from a TenantControlPlane whose name
// contains a dot: the value must remain a single identifier.
func TestQuotePostgreSQLIdentifierPreservesDottedNames(t *testing.T) {
	if got, want := quotePostgreSQLIdentifier("ns_foo.bar"), `"ns_foo.bar"`; got != want {
		t.Fatalf("dotted identifier altered: got %q want %q", got, want)
	}

	// Confirm it interpolates into a single-identifier CREATE DATABASE.
	if got, want := fmt.Sprintf(postgresqlCreateDBStatement, quotePostgreSQLIdentifier("ns_foo.bar")), `CREATE DATABASE "ns_foo.bar"`; got != want {
		t.Fatalf("statement = %q, want %q", got, want)
	}
}

func TestQuotePostgreSQLIdentifierStripsNUL(t *testing.T) {
	if got := quotePostgreSQLIdentifier("a\x00b"); strings.ContainsRune(got, 0) {
		t.Fatalf("NUL byte survived quoting: %q", got)
	}
}
