package databasesql_test

// This file tests the public NewPool constructor of the modernc package
// exactly as an external consumer uses it (hence the _test package name).
// The pragma parameter on NewPool is public API: these tests pin down the
// contract — the default pragmas always apply, and caller pragmas replace
// the default on key collision.

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db/databasesql"
)

// queryPragmaInt runs "PRAGMA name;" on the database and returns the value
// of its single result column as an int.
func queryPragmaInt(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	var value int64
	err := db.QueryRow("PRAGMA " + name + ";").Scan(&value)
	if err != nil {
		t.Fatalf("failed to query %s: %v", name, err)
	}
	return value
}

// queryPragmaText runs "PRAGMA name;" on the database and returns the
// value of its single result column as text.
func queryPragmaText(t *testing.T, db *sql.DB, name string) string {
	t.Helper()

	var value string
	err := db.QueryRow("PRAGMA " + name + ";").Scan(&value)
	if err != nil {
		t.Fatalf("failed to query %s: %v", name, err)
	}
	return value
}

// checkDefaultPragmas verifies that the shared default pragmas are applied
// on the given connection: busy_timeout 5000 ms, journal_mode WAL,
// synchronous NORMAL, foreign_keys off.
func checkDefaultPragmas(t *testing.T, db *sql.DB) {
	t.Helper()

	if got := queryPragmaInt(t, db, "busy_timeout"); got != 5000 {
		t.Errorf("default busy_timeout = %d, want 5000", got)
	}
	if got := queryPragmaText(t, db, "journal_mode"); got != "wal" {
		t.Errorf("default journal_mode = %q, want %q", got, "wal")
	}
	if got := queryPragmaInt(t, db, "synchronous"); got != 1 {
		t.Errorf("default synchronous = %d, want 1 (NORMAL)", got)
	}
	if got := queryPragmaInt(t, db, "foreign_keys"); got != 0 {
		t.Errorf("default foreign_keys = %d, want 0 (off)", got)
	}
}

// TestNewPool verifies the pool constructor produces a usable pool on a real
// database file.
func TestNewPool(t *testing.T) {
	db, err := databasesql.NewPool(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close pool: %v", err)
		}
	})

	var one int
	err = db.QueryRow("SELECT 1;").Scan(&one)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
}

// TestNewPool_PragmasDefault verifies that a pool created without pragmas
// runs all default pragmas (see checkDefaultPragmas).
func TestNewPool_PragmasDefault(t *testing.T) {
	db, err := databasesql.NewPool(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close pool: %v", err)
		}
	})

	checkDefaultPragmas(t, db)
}

// TestNewPool_PragmasOverride verifies that a pragma passed to NewPool
// replaces the default on the same key: busy_timeout becomes 12345 ms.
func TestNewPool_PragmasOverride(t *testing.T) {
	db, err := databasesql.NewPool(filepath.Join(t.TempDir(), "test.db"), map[string]string{"busy_timeout": "12345"})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close pool: %v", err)
		}
	})

	if got := queryPragmaInt(t, db, "busy_timeout"); got != 12345 {
		t.Fatalf("busy_timeout = %d, want 12345", got)
	}
}
