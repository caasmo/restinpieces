package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// setupTestScopesDB creates and primes an in-memory SQLite database for testing.
// It creates the necessary table and populates it with the given scopes.
// It returns a connection pool and handles cleanup automatically.
func setupTestScopesDB(t *testing.T, scopes []string) *sqlitex.Pool {
	t.Helper()

	// Use an in-memory database. PoolSize: 1 ensures we operate on the same db.
	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{
		PoolSize: 1,
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	conn := pool.Get(context.Background())
	defer pool.Put(conn)

	// Create schema from embedded migrations
	schemaFS := migrations.Schema()
	sqlBytes, err := fs.ReadFile(schemaFS, "app/app_config.sql")
	if err != nil {
		t.Fatalf("Failed to read app/app_config.sql: %v", err)
	}
	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute app_config.sql: %v", err)
	}

	// Insert test data
	if len(scopes) > 0 {
		stmt, err := conn.Prepare("INSERT INTO app_config (scope, content) VALUES (?, ?);")
		if err != nil {
			t.Fatalf("Failed to prepare insert statement: %v", err)
		}
		defer stmt.Finalize()

		for _, scope := range scopes {
			stmt.BindText(1, scope)
			stmt.BindText(2, "{}") // Set a default value for the 'content' column.
			if _, err := stmt.Step(); err != nil {
				t.Fatalf("Failed to insert scope '%s': %v", scope, err)
			}
			stmt.Reset()
		}
	}

	return pool
}

func TestListScopes_Success(t *testing.T) {
	// --- Setup ---
	// Unordered and with duplicates to test the query's DISTINCT and ORDER BY clauses.
	initialScopes := []string{"scope-c", "scope-a", "scope-b", "scope-a"}
	pool := setupTestScopesDB(t, initialScopes)
	var stdout bytes.Buffer

	// --- Execute ---
	err := listScopes(&stdout, pool)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOutput := "scope-a\nscope-b\nscope-c\n"
	if got := stdout.String(); got != expectedOutput {
		t.Errorf("expected output:\n%q\ngot:\n%q", expectedOutput, got)
	}
}

func TestListScopes_Success_NoScopes(t *testing.T) {
	// --- Setup ---
	pool := setupTestScopesDB(t, []string{}) // Empty table
	var stdout bytes.Buffer

	// --- Execute ---
	err := listScopes(&stdout, pool)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() > 0 {
		t.Errorf("expected empty output, but got: %q", stdout.String())
	}
}

func TestListScopes_Failure_DbConnectionError(t *testing.T) {
	// --- Setup ---
	pool := setupTestScopesDB(t, []string{})
	// Immediately close the pool to force a connection error.
	if err := pool.Close(); err != nil {
		t.Fatalf("failed to close pool for test setup: %v", err)
	}
	var stdout bytes.Buffer

	// --- Execute ---
	err := listScopes(&stdout, pool)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}

	if !errors.Is(err, ErrDbConnection) {
		t.Errorf("expected error to wrap ErrDbConnection, got %v", err)
	}
}