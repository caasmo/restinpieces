package zombiezen

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestDB creates a new in-memory SQLite database and applies all schemas.
func newTestDB(t *testing.T) *Db {
	t.Helper()

	//  each connection in the pool gets its own separate in-memory database
	//  instance. we need to make sure we only have one
	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{
		PoolSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}

	t.Cleanup(func() {
		err := pool.Close()
		if err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get db connection: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := migrations.Schema()

	// Directly read and execute the app_config.sql file we need
	sqlBytes, err := fs.ReadFile(schemaFS, "app/app_config.sql")
	if err != nil {
		t.Fatalf("Failed to read app_config.sql: %v", err)
	}

	t.Logf("Applying migration: app/app_config.sql")
	//t.Logf("Migration SQL contents:\n%s", string(sqlBytes))
	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute app_config.sql: %v", err)
	}

	db, err := New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

func TestConfigSuite(t *testing.T) {
	dbtest.ConfigSuite{Db: newTestDB(t)}.RunAll(t)
}

func TestPath_FileDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{})
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	db := &Db{pool: pool}

	p := db.Path()
	if p != dbPath {
		t.Errorf("expected path '%s', got '%s'", dbPath, p)
	}
}

func TestPath_InMemory(t *testing.T) {
	db := newTestDB(t)
	path := db.Path()
	if path != "" {
		t.Errorf("expected empty path for in-memory db, got '%s'", path)
	}
}
