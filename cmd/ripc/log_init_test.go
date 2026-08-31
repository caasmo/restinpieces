package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// MockLogInitSecureStore is a test-only implementation of config.SecureStore for log init tests.
type MockLogInitSecureStore struct {
	data          []byte
	forceGetError bool
	savedData     []byte
}

func (m *MockLogInitSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.forceGetError {
		return nil, "", errors.New("forced get error")
	}
	return m.data, "toml", nil
}

func (m *MockLogInitSecureStore) Save(scope string, data []byte, format string, description string) error {
	m.savedData = data
	return nil
}

func TestUpdateLogPathInConfig(t *testing.T) {
	t.Run("SuccessNewConfig", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{}
		err := updateLogPathInConfig(mockStore, "/custom/logs.db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var cfg config.Config
		err = toml.Unmarshal(mockStore.savedData, &cfg)
		if err != nil {
			t.Fatalf("failed to unmarshal saved config: %v", err)
		}
		if cfg.Log.Batch.DbPath != "/custom/logs.db" {
			t.Errorf("expected db_path /custom/logs.db, got %q", cfg.Log.Batch.DbPath)
		}
	})
}

func TestLogInit(t *testing.T) {
	t.Run("MissingPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("")}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logInit(ui, mockStore, "/tmp/app.db", "")
		if err == nil {
			t.Fatal("expected error when log path is missing, got nil")
		}
	})

	t.Run("SuccessCustomPath", func(t *testing.T) {
		tempDir := t.TempDir()
		appDbPath := filepath.Join(tempDir, "app.db")
		customLogDbPath := filepath.Join(tempDir, "custom-logs.db")

		mockStore := &MockLogInitSecureStore{data: []byte("")}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logInit(ui, mockStore, appDbPath, customLogDbPath)
		if err != nil {
			t.Fatalf("logInit failed: %v", err)
		}

		if _, err := os.Stat(customLogDbPath); os.IsNotExist(err) {
			t.Fatalf("custom log database file was not created at %s", customLogDbPath)
		}

		var cfg config.Config
		err = toml.Unmarshal(mockStore.savedData, &cfg)
		if err != nil {
			t.Fatalf("failed to unmarshal saved config: %v", err)
		}
		if cfg.Log.Batch.DbPath != customLogDbPath {
			t.Errorf("expected log.batch.db_path %q, got %q", customLogDbPath, cfg.Log.Batch.DbPath)
		}
	})

	t.Run("FailureOnCreatePool", func(t *testing.T) {
		logDbPath := "/dev/null/logs.db"
		mockStore := &MockLogInitSecureStore{}

		ui := UI{Out: io.Discard, Err: io.Discard}
		err := logInit(ui, mockStore, "/tmp/app.db", logDbPath)

		if !errors.Is(err, ErrDbConnection) {
			t.Fatalf("expected error to wrap ErrDbConnection, got %v", err)
		}
	})
}
