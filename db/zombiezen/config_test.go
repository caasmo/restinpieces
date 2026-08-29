package zombiezen

import (
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestDB creates a new in-memory SQLite database and applies all schemas.
func newTestDB(t *testing.T) *Db {
	return newTestDb(t, "app/app_config.sql")
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
