package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// MockMigrateSecureStore is a test-only implementation of config.SecureStore for migrate command tests.
type MockMigrateSecureStore struct {
	storedTOML     []byte
	forceGetError  bool
	forceSaveError bool
	savedTOML      []byte
	saveDescs      []string
}

func NewMockMigrateSecureStore() *MockMigrateSecureStore {
	return &MockMigrateSecureStore{}
}

func (m *MockMigrateSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.forceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	if m.storedTOML == nil {
		return nil, "", nil
	}
	return m.storedTOML, "toml", nil
}

func (m *MockMigrateSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.forceSaveError {
		return fmt.Errorf("forced save error: %w", ErrSecureStoreSave)
	}
	m.savedTOML = data
	m.saveDescs = append(m.saveDescs, description)
	return nil
}

// TestMigrateConfig_NoExistingConfig verifies fresh init when no config is stored.
func TestMigrateConfig_NoExistingConfig(t *testing.T) {
	mockStore := NewMockMigrateSecureStore()
	// storedTOML remains nil → Get returns nil bytes, no error → fresh init path

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := migrateConfig(ui, mockStore)

	if err != nil {
		t.Fatalf("migrateConfig() returned unexpected error: %v", err)
	}

	if mockStore.savedTOML == nil {
		t.Fatal("expected Save to be called, but it was not")
	}

	// Verify saved config is the default config
	var savedConfig config.Config
	unmarshalErr := toml.Unmarshal(mockStore.savedTOML, &savedConfig)
	if unmarshalErr != nil {
		t.Fatalf("failed to unmarshal saved config: %v", unmarshalErr)
	}
	expectedConfig := config.NewDefaultConfig()
	if savedConfig.Server.Addr != expectedConfig.Server.Addr {
		t.Errorf("saved config server.addr = %q, want %q", savedConfig.Server.Addr, expectedConfig.Server.Addr)
	}

	output := stderr.String()
	if output == "" {
		t.Error("expected output message, got empty")
	}
}

// TestMigrateConfig_ExistingConfigMerges verifies merge when Get succeeds.
func TestMigrateConfig_ExistingConfigMerges(t *testing.T) {
	mockStore := NewMockMigrateSecureStore()
	override := `[server]
addr = ":9090"
`
	mockStore.storedTOML = []byte(override)

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := migrateConfig(ui, mockStore)

	if err != nil {
		t.Fatalf("migrateConfig() returned unexpected error: %v", err)
	}

	if mockStore.savedTOML == nil {
		t.Fatal("expected Save to be called, but it was not")
	}

	// Verify merge: stored override should be kept, defaults should fill missing
	var savedConfig config.Config
	unmarshalErr := toml.Unmarshal(mockStore.savedTOML, &savedConfig)
	if unmarshalErr != nil {
		t.Fatalf("failed to unmarshal saved config: %v", unmarshalErr)
	}

	if savedConfig.Server.Addr != ":9090" {
		t.Errorf("saved config server.addr = %q, want %q", savedConfig.Server.Addr, ":9090")
	}
	expectedConfig := config.NewDefaultConfig()
	if savedConfig.PublicDir != expectedConfig.PublicDir {
		t.Errorf("saved config PublicDir = %q, want default %q", savedConfig.PublicDir, expectedConfig.PublicDir)
	}

	output := stderr.String()
	if output == "" {
		t.Error("expected output message, got empty")
	}
}

// TestMigrateConfig_SaveError verifies error on save failure.
func TestMigrateConfig_SaveError(t *testing.T) {
	mockStore := NewMockMigrateSecureStore()
	mockStore.forceSaveError = true

	ui := UI{Out: io.Discard, Err: io.Discard}
	err := migrateConfig(ui, mockStore)

	if err == nil {
		t.Fatal("migrateConfig() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("migrateConfig() error = %v, want error wrapping %v", err, ErrSecureStoreSave)
	}
}

// TestMigrateConfig_BadTOML verifies error on unmarshal failure.
func TestMigrateConfig_BadTOML(t *testing.T) {
	mockStore := NewMockMigrateSecureStore()
	mockStore.storedTOML = []byte("this is not valid TOML {{{")

	ui := UI{Out: io.Discard, Err: io.Discard}
	err := migrateConfig(ui, mockStore)

	if err == nil {
		t.Fatal("migrateConfig() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("migrateConfig() error = %v, want error wrapping %v", err, ErrConfigUnmarshal)
	}
}

// TestHandleMigrateCommand_Help verifies that -h prints usage to stdout and
// returns nil instead of an error.
func TestHandleMigrateCommand_Help(t *testing.T) {
	mockStore := NewMockMigrateSecureStore()
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleMigrateCommand(mockStore, []string{"-h"}, ui)

	if err != nil {
		t.Fatalf("expected no error for -h, got %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Usage:")) {
		t.Errorf("expected usage on stdout, got: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %q", stderr.String())
	}
}
