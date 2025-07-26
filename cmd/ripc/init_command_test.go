package main

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// MockInitSecureStore is a test-only implementation of config.SecureStore for init command tests.
type MockInitSecureStore struct {
	data           map[string][]byte
	format         string
	saveHistory    []string
	ForceSaveError bool
}

func NewMockInitSecureStore() *MockInitSecureStore {
	return &MockInitSecureStore{
		data: make(map[string][]byte),
	}
}

// Get is not used by the init command, but is required by the interface.
func (m *MockInitSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	return nil, "", nil
}

func (m *MockInitSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("forced save error: %w", ErrSecureStoreSave)
	}
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

// TestInitializeConfig_Success verifies successful initialization.
func TestInitializeConfig_Success(t *testing.T) {
	mockStore := NewMockInitSecureStore()

	err := initializeConfig(io.Discard, mockStore)

	if err != nil {
		t.Fatalf("initializeConfig() returned unexpected error: %v", err)
	}

	// Verify save description
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != "Initial default configuration" {
		t.Errorf("expected save description 'Initial default configuration', got %v", mockStore.saveHistory)
	}

	// Verify data was saved for the correct scope
	savedData, ok := mockStore.data[config.ScopeApplication]
	if !ok {
		t.Fatalf("data was not saved for the application scope")
	}

	// Verify data saved is the default config by unmarshaling and comparing a known value.
	// This is more robust than comparing the entire struct, which can have issues with nil vs empty maps.
	var savedConfig config.Config
	if err := toml.Unmarshal(savedData, &savedConfig); err != nil {
		t.Fatalf("failed to unmarshal saved config from store: %v", err)
	}

	expectedConfig := config.NewDefaultConfig()
	if savedConfig.Server.Addr != expectedConfig.Server.Addr {
		t.Errorf("saved config does not appear to match default config. Got server addr %q, want %q",
			savedConfig.Server.Addr, expectedConfig.Server.Addr)
	}
}

// TestInitializeConfig_Failure_StoreSaveError tests failure on a store save error.
func TestInitializeConfig_Failure_StoreSaveError(t *testing.T) {
	mockStore := NewMockInitSecureStore()
	mockStore.ForceSaveError = true

	err := initializeConfig(io.Discard, mockStore)

	if err == nil {
		t.Fatal("initializeConfig() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("initializeConfig() error = %v, want error wrapping %v", err, ErrSecureStoreSave)
	}
}