package zombiezen

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

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
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	conn := pool.Get(context.Background())
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

	return &Db{pool: pool}
}

func TestGetAndInsertConfig(t *testing.T) {
	db := newTestDB(t)

	// 1. Verify table is initially empty
	content, format, err := db.GetConfig("app", 0)
	if err != nil {
		t.Fatalf("GetConfig from empty table failed: %v", err)
	}
	if content != nil {
		t.Errorf("expected nil content from empty table, got %s", content)
	}
	if format != "" {
		t.Errorf("expected empty format from empty table, got %s", format)
	}

	// 2. Insert configurations
	tests := []struct {
		scope       string
		content     []byte
		format      string
		description string
	}{
		{"app", []byte("v1"), "toml", "first version"},
		{"other", []byte("vA"), "json", "other scope"},
		{"app", []byte("v2"), "toml", "second version"},
	}

	for _, tt := range tests {
		err := db.InsertConfig(tt.scope, tt.content, tt.format, tt.description)
		if err != nil {
			t.Fatalf("InsertConfig failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 3. Test GetConfig
	t.Run("GetLatestAppConfig", func(t *testing.T) {
		content, format, err := db.GetConfig("app", 0)
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if format != "toml" {
			t.Errorf("expected format 'toml', got '%s'", format)
		}
		if !bytes.Equal(content, []byte("v2")) {
			t.Errorf("expected content 'v2', got '%s'", content)
		}
	})

	t.Run("GetPreviousAppConfig", func(t *testing.T) {
		content, format, err := db.GetConfig("app", 1)
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if format != "toml" {
			t.Errorf("expected format 'toml', got '%s'", format)
		}
		if !bytes.Equal(content, []byte("v1")) {
			t.Errorf("expected content 'v1', got '%s'", content)
		}
	})

	t.Run("GetOtherScopeConfig", func(t *testing.T) {
		content, format, err := db.GetConfig("other", 0)
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if format != "json" {
			t.Errorf("expected format 'json', got '%s'", format)
		}
		if !bytes.Equal(content, []byte("vA")) {
			t.Errorf("expected content 'vA', got '%s'", content)
		}
	})

	// 4. Test edge cases
	t.Run("NonExistentScope", func(t *testing.T) {
		content, _, err := db.GetConfig("nonexistent", 0)
		if err != nil {
			t.Fatalf("GetConfig failed for nonexistent scope: %v", err)
		}
		if content != nil {
			t.Errorf("expected nil content for nonexistent scope, got %s", content)
		}
	})

	t.Run("GenerationOutOfBounds", func(t *testing.T) {
		content, _, err := db.GetConfig("app", 2)
		if err != nil {
			t.Fatalf("GetConfig failed for out-of-bounds generation: %v", err)
		}
		if content != nil {
			t.Errorf("expected nil content for out-of-bounds generation, got %s", content)
		}
	})
}

func TestPath_FileDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{})
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}
	defer pool.Close()

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
