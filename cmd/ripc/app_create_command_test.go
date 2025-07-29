package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestPool creates a new in-memory SQLite database pool for testing.
// It does not apply any migrations.
func newTestPool(t *testing.T) *sqlitex.Pool {
	t.Helper()

	// Each connection in the pool gets its own separate in-memory database
	// instance. We need to make sure we only have one for the test.
	pool, err := sqlitex.NewPool("file::memory:?mode=memory", sqlitex.PoolOptions{
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

	return pool
}

// Mock for SecureStore to test saveConfig
type MockAppCreateSecureStore struct {
	saveCalled     bool
	saveData       []byte
	saveFormat     string
	saveDesc       string
	forceSaveError bool
}

func (m *MockAppCreateSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	panic("not implemented")
}

func (m *MockAppCreateSecureStore) Save(scope string, data []byte, format string, description string) error {
	m.saveCalled = true
	m.saveData = data
	m.saveFormat = format
	m.saveDesc = description
	if m.forceSaveError {
		return ErrSecureStoreSave
	}
	return nil
}

func TestSaveConfig(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &MockAppCreateSecureStore{}
		testData := []byte("test-config")
		var stdout bytes.Buffer

		err := saveConfig(&stdout, mockStore, testData)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !mockStore.saveCalled {
			t.Error("expected Save to be called, but it wasn't")
		}
		if !bytes.Equal(mockStore.saveData, testData) {
			t.Errorf("expected saved data %q, got %q", testData, mockStore.saveData)
		}
		if mockStore.saveFormat != "toml" {
			t.Errorf("expected format 'toml', got %q", mockStore.saveFormat)
		}
		if mockStore.saveDesc != "Initial default configuration" {
			t.Errorf("expected description 'Initial default configuration', got %q", mockStore.saveDesc)
		}
		if stdout.String() != "Saving initial configuration...\n" {
			t.Errorf("unexpected output: %q", stdout.String())
		}
	})

	t.Run("Failure", func(t *testing.T) {
		mockStore := &MockAppCreateSecureStore{forceSaveError: true}
		testData := []byte("test-config")

		err := saveConfig(io.Discard, mockStore, testData)

		if !errors.Is(err, ErrSecureStoreSave) {
			t.Fatalf("expected error to wrap ErrSecureStoreSave, got %v", err)
		}
	})
}

func TestRunMigrations(t *testing.T) {
	pool := newTestPool(t)
	var stdout bytes.Buffer

	err := runMigrations(&stdout, pool)
	if err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	// Verify that the tables were created
	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get connection from pool: %v", err)
	}
	defer pool.Put(conn)

	expectedTables := []string{"app_config", "users", "job_queue"}
	for _, table := range expectedTables {
		var count int
		err := sqlitex.Execute(conn, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?;", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
			Args: []any{table},
		})
		if err != nil {
			t.Fatalf("failed to query for table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("expected table %s to be created, but it wasn't", table)
		}
	}
}

func TestCreateApplication(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		pool := newTestPool(t)
		mockStore := &MockAppCreateSecureStore{}
		var stdout bytes.Buffer

		err := createApplication(&stdout, mockStore, pool, "test.db")
		if err != nil {
			t.Fatalf("createApplication failed: %v", err)
		}

		// Verify migrations ran
		conn, err := pool.Take(context.Background())
		if err != nil {
			t.Fatalf("failed to get connection from pool: %v", err)
		}
		defer pool.Put(conn)
		var count int
		err = sqlitex.Execute(conn, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users';", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to query for users table: %v", err)
		}
		if count != 1 {
			t.Error("expected migrations to be run, but users table not found")
		}

		// Verify config was saved
		if !mockStore.saveCalled {
			t.Error("expected Save to be called, but it wasn't")
		}
		var savedCfg config.Config
		if err := toml.Unmarshal(mockStore.saveData, &savedCfg); err != nil {
			t.Fatalf("failed to unmarshal saved config: %v", err)
		}
		if savedCfg.Server.Addr != config.NewDefaultConfig().Server.Addr {
			t.Error("saved config does not appear to be the default config")
		}
	})

	t.Run("FailureOnSave", func(t *testing.T) {
		pool := newTestPool(t)
		mockStore := &MockAppCreateSecureStore{forceSaveError: true}

		err := createApplication(io.Discard, mockStore, pool, "test.db")

		if !errors.Is(err, ErrSecureStoreSave) {
			t.Fatalf("expected error to wrap ErrSecureStoreSave, got %v", err)
		}
	})
}
