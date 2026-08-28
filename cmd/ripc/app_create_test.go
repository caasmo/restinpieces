package main

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// MockAppCreateSecureStore is a test-only implementation of config.SecureStore
// for app create tests.
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
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := saveConfig(ui, mockStore, testData)

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
		if stderr.String() != "Saving initial configuration...\n" {
			t.Errorf("unexpected output: %q", stderr.String())
		}
	})

	t.Run("Failure", func(t *testing.T) {
		mockStore := &MockAppCreateSecureStore{forceSaveError: true}
		testData := []byte("test-config")

		ui := UI{Out: io.Discard, Err: io.Discard}
		err := saveConfig(ui, mockStore, testData)

		if !errors.Is(err, ErrSecureStoreSave) {
			t.Fatalf("expected error to wrap ErrSecureStoreSave, got %v", err)
		}
	})
}

func TestApplyAppSchema(t *testing.T) {
	db := newTestAppDb(t)

	if err := db.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}
}

func TestCreateApplication(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		db := newTestAppDb(t)
		mockStore := &MockAppCreateSecureStore{}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := createApplication(ui, mockStore, db, "test.db")
		if err != nil {
			t.Fatalf("createApplication failed: %v", err)
		}

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
		db := newTestAppDb(t)
		mockStore := &MockAppCreateSecureStore{forceSaveError: true}

		ui := UI{Out: io.Discard, Err: io.Discard}
		err := createApplication(ui, mockStore, db, "test.db")

		if !errors.Is(err, ErrSecureStoreSave) {
			t.Fatalf("expected error to wrap ErrSecureStoreSave, got %v", err)
		}
	})
}
