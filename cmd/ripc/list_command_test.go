package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// setupTestDB is a test helper function that creates an in-memory SQLite
// database, migrates the schema, and optionally seeds it with data. It returns a
// connection pool and a cleanup function to close the database connection.
func setupTestDB(t *testing.T, configs [][2]string) *sqlitex.Pool {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{PoolSize: 1})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	conn := pool.Get(context.Background())
	defer pool.Put(conn)

	schemaFS := migrations.Schema()
	sqlBytes, err := fs.ReadFile(schemaFS, "app/app_config.sql")
	if err != nil {
		t.Fatalf("failed to read app/app_config.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("failed to execute app_config.sql: %v", err)
	}

	if len(configs) > 0 {
		stmt, err := conn.Prepare("INSERT INTO app_config (scope, content) VALUES (?, ?);")
		if err != nil {
			t.Fatalf("failed to prepare insert statement: %v", err)
		}
		defer stmt.Finalize()

		for _, config := range configs {
			stmt.BindText(1, config[0])
			stmt.BindText(2, config[1])
			if _, err := stmt.Step(); err != nil {
				t.Fatalf("failed to insert config with scope '%s': %v", config[0], err)
			}
			stmt.Reset()
		}
	}

	return pool
}

func TestListItems_Success(t *testing.T) {
	configs := [][2]string{
		{"scope-a", "content-a"},
		{"scope-b", "content-b"},
		{"scope-a", "content-c"},
	}
	pool := setupTestDB(t, configs)

	var stdout bytes.Buffer
	count, err := listItems(&stdout, pool, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}

	output := stdout.String()
	if !strings.Contains(output, "scope-a") || !strings.Contains(output, "scope-b") {
		t.Errorf("output does not contain expected scopes: %s", output)
	}
}

func TestListItems_Success_NoItems(t *testing.T) {
	pool := setupTestDB(t, nil)

	var stdout bytes.Buffer
	count, err := listItems(&stdout, pool, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 items, got %d", count)
	}
}

func TestListItems_Failure_DbConnectionError(t *testing.T) {
	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{PoolSize: 1})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	pool.Close() // Close the pool to trigger a connection error

	var stdout bytes.Buffer
	_, err = listItems(&stdout, pool, "")

	if !errors.Is(err, ErrDbConnection) {
		t.Errorf("expected ErrDbConnection, got %v", err)
	}
}

func TestListItems_Failure_QueryError(t *testing.T) {
	pool := setupTestDB(t, nil)

	// Get a connection from the pool to modify the schema.
	conn := pool.Get(context.Background())
	// Intentionally drop the table to cause a query error in the function under test.
	if err := sqlitex.ExecuteTransient(conn, "DROP TABLE app_config;", nil); err != nil {
		t.Fatalf("failed to drop table: %v", err)
	}
	// IMPORTANT: Return the connection to the pool so the function under test can use it.
	pool.Put(conn)

	var stdout bytes.Buffer
	_, err := listItems(&stdout, pool, "")

	if err == nil {
		t.Fatal("expected an error, but got nil")
	}

	// The error should be ErrQueryPrepare because conn.Prepare will fail.
	if !errors.Is(err, ErrQueryPrepare) {
		t.Errorf("expected error to wrap ErrQueryPrepare, got %v", err)
	}
}
