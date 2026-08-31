package zombiezen_test

// This file tests the public NewPool constructor of the zombiezen package
// exactly as an external consumer uses it (hence the _test package name).
// The pragma parameter on NewPool is public API: these tests pin down the
// contract — the default pragmas always apply, and caller pragmas run after
// them and override them on key collision.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// queryPragmaInt runs "PRAGMA name;" on the connection and returns the value
// of its single result column as an int.
func queryPragmaInt(t *testing.T, conn *sqlite.Conn, name string) int64 {
	t.Helper()

	stmt, err := conn.Prepare("PRAGMA " + name + ";")
	if err != nil {
		t.Fatalf("failed to prepare %s query: %v", name, err)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil {
			t.Errorf("failed to finalize %s statement: %v", name, ferr)
		}
	}()

	row, err := stmt.Step()
	if err != nil {
		t.Fatalf("failed to step %s query: %v", name, err)
	}
	if !row {
		t.Fatalf("%s query returned no row", name)
	}
	return stmt.ColumnInt64(0)
}

// queryPragmaText runs "PRAGMA name;" on the connection and returns the
// value of its single result column as text.
func queryPragmaText(t *testing.T, conn *sqlite.Conn, name string) string {
	t.Helper()

	stmt, err := conn.Prepare("PRAGMA " + name + ";")
	if err != nil {
		t.Fatalf("failed to prepare %s query: %v", name, err)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil {
			t.Errorf("failed to finalize %s statement: %v", name, ferr)
		}
	}()

	row, err := stmt.Step()
	if err != nil {
		t.Fatalf("failed to step %s query: %v", name, err)
	}
	if !row {
		t.Fatalf("%s query returned no row", name)
	}
	return stmt.ColumnText(0)
}

// checkDefaultPragmas verifies that the shared default pragmas are applied
// on the given connection: busy_timeout 5000 ms, journal_mode WAL,
// synchronous NORMAL, foreign_keys off.
func checkDefaultPragmas(t *testing.T, conn *sqlite.Conn) {
	t.Helper()

	if got := queryPragmaInt(t, conn, "busy_timeout"); got != 5000 {
		t.Errorf("default busy_timeout = %d, want 5000", got)
	}
	if got := queryPragmaText(t, conn, "journal_mode"); got != "wal" {
		t.Errorf("default journal_mode = %q, want %q", got, "wal")
	}
	if got := queryPragmaInt(t, conn, "synchronous"); got != 1 {
		t.Errorf("default synchronous = %d, want 1 (NORMAL)", got)
	}
	if got := queryPragmaInt(t, conn, "foreign_keys"); got != 0 {
		t.Errorf("default foreign_keys = %d, want 0 (off)", got)
	}
}

// TestNewPool verifies the pool constructor produces a usable pool on a real
// database file.
func TestNewPool(t *testing.T) {
	pool, err := zombiezen.NewPool(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to take connection: %v", err)
	}
	defer pool.Put(conn)

	err = sqlitex.Execute(conn, "SELECT 1;", nil)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
}

// TestNewPool_PragmasDefault verifies that a pool created without pragmas
// runs all default pragmas (see checkDefaultPragmas).
func TestNewPool_PragmasDefault(t *testing.T) {
	pool, err := zombiezen.NewPool(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to take connection: %v", err)
	}
	defer pool.Put(conn)

	checkDefaultPragmas(t, conn)
}

// TestNewPool_PragmasOverride verifies that a pragma passed to NewPool runs
// after the defaults and overrides them: busy_timeout becomes 12345 ms.
func TestNewPool_PragmasOverride(t *testing.T) {
	pool, err := zombiezen.NewPool(filepath.Join(t.TempDir(), "test.db"), "PRAGMA busy_timeout = 12345")
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to take connection: %v", err)
	}
	defer pool.Put(conn)

	if got := queryPragmaInt(t, conn, "busy_timeout"); got != 12345 {
		t.Fatalf("busy_timeout = %d, want 12345", got)
	}
}
