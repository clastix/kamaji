// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"strings"
	"testing"
)

// TestQuoteMySQLIdentifierNeutralisesInjection ensures a malicious identifier
// cannot break out of the backtick quoting and inject additional SQL, which is
// the regression behind the dataStoreSchema/dataStoreUsername SQL injection.
func TestQuoteMySQLIdentifierNeutralisesInjection(t *testing.T) {
	const bt = "`"

	// A tenant-controlled value attempting to stack a DROP DATABASE statement by
	// closing the identifier with a backtick.
	payload := "x" + bt + "; DROP DATABASE victim; -- "

	// Every embedded backtick is doubled, so the whole payload stays a single
	// quoted identifier instead of terminating it early.
	want := bt + "x" + bt + bt + "; DROP DATABASE victim; -- " + bt
	if got := quoteMySQLIdentifier(payload); got != want {
		t.Fatalf("quoteMySQLIdentifier(%q) = %q, want %q", payload, got, want)
	}

	// Structural safety property: stripping the outer backticks and collapsing
	// the doubled backticks must leave no lone backtick that could close the
	// identifier prematurely.
	inner := want[1 : len(want)-1]
	if strings.Contains(strings.ReplaceAll(inner, bt+bt, ""), bt) {
		t.Fatalf("a lone backtick survived quoting: %q", want)
	}
}

func TestEscapeMySQLStringNeutralisesInjection(t *testing.T) {
	payload := "p'; DROP DATABASE victim; -- "

	// The single quote that would terminate the literal is doubled ('') rather
	// than backslash-escaped, so the payload cannot escape the surrounding '...'
	// literal under any sql_mode, including NO_BACKSLASH_ESCAPES.
	want := `p''; DROP DATABASE victim; -- `
	if got := escapeMySQLString(payload); got != want {
		t.Fatalf("escapeMySQLString(%q) = %q, want %q", payload, got, want)
	}
}

// TestEscapeMySQLStringQuoteEscapingIsModeIndependent guards against a
// regression back to backslash-escaping the quote (\'), which is unsafe when the
// server runs with NO_BACKSLASH_ESCAPES. Quotes must be doubled and backslashes
// must be doubled independently.
func TestEscapeMySQLStringQuoteEscapingIsModeIndependent(t *testing.T) {
	// Quote-only input: the sole transformation is '' doubling, so an exact match
	// unambiguously proves the quote is not backslash-escaped.
	if got, want := escapeMySQLString("a'b'c"), "a''b''c"; got != want {
		t.Fatalf("escapeMySQLString(%q) = %q, want %q", "a'b'c", got, want)
	}

	// Combined quote + backslash: both are doubled, still without producing a
	// backslash-escaped quote.
	if got, want := escapeMySQLString(`'\`), `''\\`; got != want {
		t.Fatalf("escapeMySQLString(%q) = %q, want %q", `'\`, got, want)
	}
}

func TestEscapeMySQLStringEscapesBackslash(t *testing.T) {
	// A backslash must be doubled, otherwise `\'` could be smuggled in.
	if got, want := escapeMySQLString(`a\`), `a\\`; got != want {
		t.Fatalf("escapeMySQLString(%q) = %q, want %q", `a\`, got, want)
	}
}

func TestQuoteMySQLIdentifierStripsNUL(t *testing.T) {
	if got := quoteMySQLIdentifier("a\x00b"); strings.ContainsRune(got, 0) {
		t.Fatalf("NUL byte survived quoting: %q", got)
	}
}

// TestQuoteMySQLIdentifierPreservesValidIdentifiers guards the SHOW GRANTS
// comparison in GrantPrivilegesExists: well-formed identifiers must keep the
// exact backtick-wrapped shape MySQL itself emits.
func TestQuoteMySQLIdentifierPreservesValidIdentifiers(t *testing.T) {
	if got, want := quoteMySQLIdentifier("tenant_namespace_cp"), "`tenant_namespace_cp`"; got != want {
		t.Fatalf("valid identifier altered: got %q want %q", got, want)
	}
}
