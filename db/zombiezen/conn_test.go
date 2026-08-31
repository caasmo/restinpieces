package zombiezen_test

// This file tests the public NewConn constructor of the zombiezen package
// exactly as an external consumer uses it (hence the _test package name).
// The pragma parameter on NewConn is public API: these tests pin down the
// contract — the default pragmas always apply, and caller pragmas run after
// them and override them on key collision.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestDbFile creates an empty database file and returns its path.
// NewConn opens with OpenReadWrite and no OpenCreate, so the file must
// pre-exist.
func newTestDbFile(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	err := os.WriteFile(dbPath, nil, 0o600)
	if err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}
	return dbPath
}

// TestNewConn verifies the single-connection constructor on an existing
// database file (OpenCreate is not used, so the file must pre-exist).
func TestNewConn(t *testing.T) {
	conn, err := zombiezen.NewConn(newTestDbFile(t))
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	err = sqlitex.Execute(conn, "SELECT 1;", nil)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
}

// TestNewConn_PragmasDefault verifies that a connection opened without
// pragmas runs all default pragmas (see checkDefaultPragmas).
func TestNewConn_PragmasDefault(t *testing.T) {
	conn, err := zombiezen.NewConn(newTestDbFile(t))
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	checkDefaultPragmas(t, conn)
}

// TestNewConn_PragmasOverride verifies that a pragma passed to NewConn runs
// after the defaults and overrides them: busy_timeout becomes 12345 ms.
func TestNewConn_PragmasOverride(t *testing.T) {
	conn, err := zombiezen.NewConn(newTestDbFile(t), "PRAGMA busy_timeout = 12345")
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	if got := queryPragmaInt(t, conn, "busy_timeout"); got != 12345 {
		t.Fatalf("busy_timeout = %d, want 12345", got)
	}
}
