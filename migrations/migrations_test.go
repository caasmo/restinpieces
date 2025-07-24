package migrations

import (
	"context"
	"io/fs"
	"reflect"
	"sort"
	"testing"

	"zombiezen.com/go/sqlite/sqlitex"
)

// TestSchemaAccess verifies that all expected .sql files are embedded correctly.
func TestSchemaAccess(t *testing.T) {
	expectedFiles := []string{
		"app/app_config.sql",
		"app/job_queue.sql",
		"app/users.sql",
		"log/logs.sql",
	}

	var foundFiles []string
	schemaFS := Schema()

	err := fs.WalkDir(schemaFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			foundFiles = append(foundFiles, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk embedded schema files: %v", err)
	}

	sort.Strings(expectedFiles)
	sort.Strings(foundFiles)

	if !reflect.DeepEqual(expectedFiles, foundFiles) {
		t.Errorf("mismatch in embedded schema files.\nGot:  %v\nWant: %v", foundFiles, expectedFiles)
	}
}

// TestApplySchemas creates an in-memory SQLite database and applies all embedded
// .sql schema files to ensure they are syntactically valid.
func TestApplySchemas(t *testing.T) {
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

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get db connection: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := Schema()

	err = fs.WalkDir(schemaFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil // Skip directories
		}

		t.Run("Applying_"+path, func(t *testing.T) {
			sqlBytes, err := fs.ReadFile(schemaFS, path)
			if err != nil {
				t.Fatalf("failed to read embedded migration file %s: %v", path, err)
			}

			if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
				t.Fatalf("failed to execute migration file %s: %v", path, err)
			}
		})
		return nil
	})

	if err != nil {
		t.Fatalf("error walking schema directory: %v", err)
	}
}
