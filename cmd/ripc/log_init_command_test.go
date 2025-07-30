package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// MockLogInitSecureStore is a test-only implementation of config.SecureStore for log init tests.
type MockLogInitSecureStore struct {
	data          []byte
	forceGetError bool
}

func (m *MockLogInitSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.forceGetError {
		return nil, "", errors.New("forced get error")
	}
	// The production code handles a nil-error with empty bytes as a valid "not found" case.
	return m.data, "toml", nil
}

func (m *MockLogInitSecureStore) Save(scope string, data []byte, format string, description string) error {
	panic("not implemented for log init tests")
}

// newLogTestPool creates a new in-memory SQLite database pool for testing migrations.
func newLogTestPool(t *testing.T) *sqlitex.Pool {
	t.Helper()
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

func TestGetLogDbPathFromConfig(t *testing.T) {
	appDbDir := t.TempDir()
	appDbPath := filepath.Join(appDbDir, "app.db")

	testCases := []struct {
		name                string
		mockStore           config.SecureStore
		expectedPath        string
		expectedUsedDefault bool
		expectedErr         error
	}{
		{
			name: "SuccessFromConfig",
			mockStore: &MockLogInitSecureStore{
				data: []byte(`[log.batch]
  db_path = "/custom/path/logs.db"
`),
			},
			expectedPath:        "/custom/path/logs.db",
			expectedUsedDefault: false,
			expectedErr:         nil,
		},
		{
			name: "SuccessDefaultPath",
			mockStore: &MockLogInitSecureStore{
				data: []byte(`[server]`), // Config exists but no log path
			},
			expectedPath:        filepath.Join(appDbDir, defaultLogFilename),
			expectedUsedDefault: true,
			expectedErr:         nil,
		},
		{
			name: "SuccessDefaultOnGetError",
			mockStore: &MockLogInitSecureStore{
				forceGetError: true,
			},
			expectedPath:        filepath.Join(appDbDir, defaultLogFilename),
			expectedUsedDefault: true,
			expectedErr:         nil,
		},
		{
			name: "FailureMalformedConfig",
			mockStore: &MockLogInitSecureStore{
				data: []byte(`[log.batch`), // Malformed TOML
			},
			expectedPath:        "",
			expectedUsedDefault: false,
			expectedErr:         ErrGetLogDbPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, usedDefault, err := getLogDbPathFromConfig(tc.mockStore, appDbPath)

			if tc.expectedErr != nil {
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error to wrap %v, but got %v", tc.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tc.expectedPath {
				t.Errorf("expected path %q, got %q", tc.expectedPath, path)
			}
			if usedDefault != tc.expectedUsedDefault {
				t.Errorf("expected usedDefault to be %v, got %v", tc.expectedUsedDefault, usedDefault)
			}
		})
	}
}

func TestRunLogMigrations(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		pool := newLogTestPool(t)
		var stdout bytes.Buffer

		err := runLogMigrations(&stdout, pool)
		if err != nil {
			t.Fatalf("runLogMigrations failed: %v", err)
		}

		// Verify that the 'logs' table was created
		conn, err := pool.Take(context.Background())
		if err != nil {
			t.Fatalf("failed to get connection from pool: %v", err)
		}
		defer pool.Put(conn)

		var count int
		err = sqlitex.Execute(conn, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='logs';", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to query for table 'logs': %v", err)
		}
		if count != 1 {
			t.Errorf("expected table 'logs' to be created, but it wasn't")
		}
	})

	t.Run("FailureDbConnectionError", func(t *testing.T) {
		// Manually create a pool and close it immediately to guarantee a connection error.
		pool, err := sqlitex.NewPool("file::memory:?mode=memory", sqlitex.PoolOptions{
			PoolSize: 1,
		})
		if err != nil {
			t.Fatalf("failed to create db pool: %v", err)
		}
		if err := pool.Close(); err != nil {
			t.Fatalf("failed to close pool for test: %v", err)
		}

		err = runLogMigrations(io.Discard, pool)

		if !errors.Is(err, ErrDbConnection) {
			t.Fatalf("expected error to wrap ErrDbConnection, got %v", err)
		}
	})
}

func TestLogInit(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tempDir := t.TempDir()
		appDbPath := filepath.Join(tempDir, "app.db")
		logDbPath := filepath.Join(tempDir, defaultLogFilename)

		// Mock store that returns a config with no log path, forcing default
		mockStore := &MockLogInitSecureStore{data: []byte("")}
		var stdout bytes.Buffer

		err := logInit(&stdout, mockStore, appDbPath)
		if err != nil {
			t.Fatalf("logInit failed: %v", err)
		}

		// Verify database file was created and has the schema
		if _, err := os.Stat(logDbPath); os.IsNotExist(err) {
			t.Fatalf("log database file was not created at %s", logDbPath)
		}

		pool, err := sqlitex.NewPool(logDbPath, sqlitex.PoolOptions{})
		if err != nil {
			t.Fatalf("failed to open created log db: %v", err)
		}
		defer pool.Close()
		conn, err := pool.Take(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Put(conn)
		var count int
		err = sqlitex.Execute(conn, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='logs';", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to query created log db: %v", err)
		}
		if count != 1 {
			t.Error("logs table not found in created database")
		}
	})

	t.Run("FailureOnGetPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("[log.batch")}
		err := logInit(io.Discard, mockStore, "/tmp/app.db")
		if !errors.Is(err, ErrGetLogDbPath) {
			t.Fatalf("expected error to wrap ErrGetLogDbPath, got %v", err)
		}
	})

	t.Run("FailureOnCreatePool", func(t *testing.T) {
		// Use a path that cannot be created to cause pool creation to fail.
		logDbPath := "/dev/null/logs.db"
		configData, _ := toml.Marshal(map[string]interface{}{
			"log": map[string]interface{}{
				"batch": map[string]interface{}{
					"db_path": logDbPath,
				},
			},
		})
		mockStore := &MockLogInitSecureStore{data: configData}

		err := logInit(io.Discard, mockStore, "/tmp/app.db")

		if !errors.Is(err, ErrCreateLogDbPool) {
			t.Fatalf("expected error to wrap ErrCreateLogDbPool, got %v", err)
		}
	})
}
