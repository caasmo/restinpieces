package modernc_test

// This file tests the public NewConn constructor of the modernc package
// exactly as an external consumer uses it (hence the _test package name).
// The pragma parameter on NewConn is public API: these tests pin down the
// contract — the default pragmas always apply, and caller pragmas replace
// the default on key collision.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db/modernc"
)

// newTestDbFile creates an empty database file and returns its path.
// NewConn opens with mode=rw and no create, so the file must pre-exist.
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
// database file (the file must pre-exist; mode=rw in the DSN).
func TestNewConn(t *testing.T) {
	db, err := modernc.NewConn(newTestDbFile(t))
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	var one int
	err = db.QueryRow("SELECT 1;").Scan(&one)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
}

// TestNewConn_PragmasDefault verifies that a connection opened without
// pragmas runs all default pragmas (see checkDefaultPragmas).
func TestNewConn_PragmasDefault(t *testing.T) {
	db, err := modernc.NewConn(newTestDbFile(t))
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	checkDefaultPragmas(t, db)
}

// TestNewConn_PragmasOverride verifies that a pragma passed to NewConn
// replaces the default on the same key: busy_timeout becomes 12345 ms.
func TestNewConn_PragmasOverride(t *testing.T) {
	db, err := modernc.NewConn(newTestDbFile(t), map[string]string{"busy_timeout": "12345"})
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	if got := queryPragmaInt(t, db, "busy_timeout"); got != 12345 {
		t.Fatalf("busy_timeout = %d, want 12345", got)
	}
}

// TestNewConn_MissingFile verifies that NewConn does not create the database
// file: opening a missing file must fail.
func TestNewConn_MissingFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing.db")
	_, err := modernc.NewConn(dbPath)
	if err == nil {
		t.Fatal("expected error for missing database file, got nil")
	}
}
