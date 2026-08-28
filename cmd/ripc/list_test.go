package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite/sqlitex"
)

// setupTestDB is a test helper function that creates an in-memory SQLite
// database, applies the schema, and optionally seeds it with data. It returns a
// connection pool and a cleanup function to close the database connection.
func setupTestDB(t *testing.T, configs [][2]string) *sqlitex.Pool {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{PoolSize: 1})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Fatalf("failed to close test database: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get connection from pool: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := sql.FS()
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
		defer func() {
			if err := stmt.Finalize(); err != nil {
				t.Fatalf("failed to finalize statement: %v", err)
			}
		}()

		for _, config := range configs {
			stmt.BindText(1, config[0])
			stmt.BindText(2, config[1])
			if _, err := stmt.Step(); err != nil {
				t.Fatalf("failed to insert config with scope '%s': %v", config[0], err)
			}
			if err := stmt.Reset(); err != nil {
				t.Fatalf("failed to reset statement: %v", err)
			}
		}
	}

	return pool
}

func TestPrintConfigList_Success(t *testing.T) {
	configs := [][2]string{
		{"scope-a", "content-a"},
		{"scope-b", "content-b"},
		{"scope-a", "content-c"},
	}
	pool := setupTestDB(t, configs)
	ripcDb := newDBFromPool(pool)

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	count, err := printConfigList(ui, ripcDb, "")

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

func TestPrintConfigList_Success_NoItems(t *testing.T) {
	pool := setupTestDB(t, nil)
	ripcDb := newDBFromPool(pool)

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	count, err := printConfigList(ui, ripcDb, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 items, got %d", count)
	}
}

func TestPrintConfigList_Failure_DbConnectionError(t *testing.T) {
	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{PoolSize: 1})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("failed to close pool for test setup: %v", err)
	}
	ripcDb := newDBFromPool(pool)

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	_, err = printConfigList(ui, ripcDb, "")

	if !errors.Is(err, ErrDbConnection) {
		t.Errorf("expected ErrDbConnection, got %v", err)
	}
}

func TestPrintConfigList_Failure_QueryError(t *testing.T) {
	pool := setupTestDB(t, nil)
	ripcDb := newDBFromPool(pool)

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get connection: %v", err)
	}
	if err := sqlitex.ExecuteTransient(conn, "DROP TABLE app_config;", nil); err != nil {
		pool.Put(conn)
		t.Fatalf("failed to drop table: %v", err)
	}
	pool.Put(conn)

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	_, err = printConfigList(ui, ripcDb, "")

	if err == nil {
		t.Fatal("expected an error, but got nil")
	}

	if !errors.Is(err, ErrQueryPrepare) {
		t.Errorf("expected ErrQueryPrepare, got %v", err)
	}
}
