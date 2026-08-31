package zombiezen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"zombiezen.com/go/sqlite/sqlitex"
)

// TestNewPool verifies the default pool constructor produces a usable pool
// on a real database file.
func TestNewPool(t *testing.T) {
	pool, err := NewPool(filepath.Join(t.TempDir(), "test.db"))
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

// TestNewConn verifies the single-connection constructor on an existing
// database file (OpenCreate is not used, so the file must pre-exist).
func TestNewConn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	err := os.WriteFile(dbPath, nil, 0o600)
	if err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}

	conn, err := NewConn(dbPath)
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
