package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// MockRollbackSecureStore is a test-only implementation of config.SecureStore
// tailored for testing the rollback command.
type MockRollbackSecureStore struct {
	// Data to be returned by Get
	getData   map[string][]byte
	getFormat string

	// Data received by Save
	saveData    map[string][]byte
	saveFormat  string
	saveHistory []string

	// Error simulation
	ForceGetError  bool
	ForceSaveError bool
}

// NewMockRollbackSecureStore creates a new mock store.
func NewMockRollbackSecureStore(initialData map[string][]byte) *MockRollbackSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockRollbackSecureStore{
		getData:   initialData,
		getFormat: "toml",
		saveData:  make(map[string][]byte),
	}
}

// Get retrieves the configuration for a scope.
func (m *MockRollbackSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("%w: forced get error", ErrSecureStoreGet)
	}
	data, ok := m.getData[scope]
	if !ok {
		return nil, "", fmt.Errorf("scope not found") // Simplified for test purposes
	}
	return data, m.getFormat, nil
}

// Save updates the configuration for a scope.
func (m *MockRollbackSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("%w: forced save error", ErrSecureStoreSave)
	}
	m.saveData[scope] = data
	m.saveFormat = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

// TestRollbackConfig_Success_ExplicitScope tests a successful rollback with an explicit scope.
func TestRollbackConfig_Success_ExplicitScope(t *testing.T) {
	// --- Setup ---
	scope := "my-scope"
	generation := 2
	initialData := []byte("some-config-data")
	mockStore := NewMockRollbackSecureStore(map[string][]byte{
		scope: initialData,
	})
	var stdout bytes.Buffer

	// --- Execute ---
	err := rollbackConfig(&stdout, mockStore, scope, generation)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify data was saved correctly
	savedData, ok := mockStore.saveData[scope]
	if !ok {
		t.Fatalf("scope '%s' not found in saved data", scope)
	}
	if !bytes.Equal(savedData, initialData) {
		t.Errorf("saved data mismatch: got %q, want %q", string(savedData), string(initialData))
	}

	// Verify save description
	expectedDesc := fmt.Sprintf("Rollback to generation %d", generation)
	if len(mockStore.saveHistory) != 1 || mockStore.saveHistory[0] != expectedDesc {
		t.Errorf("description mismatch: got %q, want %q", mockStore.saveHistory[0], expectedDesc)
	}

	// Verify stdout message
	expectedOut := fmt.Sprintf("Successfully rolled back scope '%s' to generation %d\n", scope, generation)
	if stdout.String() != expectedOut {
		t.Errorf("stdout mismatch: got %q, want %q", stdout.String(), expectedOut)
	}
}

// TestRollbackConfig_Success_DefaultScope tests a successful rollback using the default scope.
func TestRollbackConfig_Success_DefaultScope(t *testing.T) {
	// --- Setup ---
	generation := 5
	expectedScope := config.ScopeApplication
	initialData := []byte("some-config-data")
	mockStore := NewMockRollbackSecureStore(map[string][]byte{
		expectedScope: initialData,
	})
	var stdout bytes.Buffer

	// --- Execute ---
	err := rollbackConfig(&stdout, mockStore, "", generation) // Empty scope triggers default

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify data was saved correctly
	savedData, ok := mockStore.saveData[expectedScope]
	if !ok {
		t.Fatalf("scope '%s' not found in saved data", expectedScope)
	}
	if !bytes.Equal(savedData, initialData) {
		t.Errorf("saved data mismatch: got %q, want %q", string(savedData), string(initialData))
	}

	// Verify save description
	expectedDesc := fmt.Sprintf("Rollback to generation %d", generation)
	if len(mockStore.saveHistory) != 1 || mockStore.saveHistory[0] != expectedDesc {
		t.Errorf("description mismatch: got %q, want %q", mockStore.saveHistory[0], expectedDesc)
	}

	// Verify stdout message
	expectedOut := fmt.Sprintf("Successfully rolled back scope '%s' to generation %d\n", expectedScope, generation)
	if stdout.String() != expectedOut {
		t.Errorf("stdout mismatch: got %q, want %q", stdout.String(), expectedOut)
	}
}

// TestRollbackConfig_Failure_InvalidGeneration tests the input validation for the generation number.
func TestRollbackConfig_Failure_InvalidGeneration(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockRollbackSecureStore(nil)
	var stdout bytes.Buffer

	// --- Execute ---
	err := rollbackConfig(&stdout, mockStore, "any-scope", 0)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrInvalidGeneration) {
		t.Errorf("expected error to wrap ErrInvalidGeneration, got %v", err)
	}
	if len(mockStore.saveData) > 0 {
		t.Error("no data should have been saved on failure")
	}
}

// TestRollbackConfig_Failure_GetError tests the failure case when retrieving the configuration fails.
func TestRollbackConfig_Failure_GetError(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockRollbackSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout bytes.Buffer

	// --- Execute ---
	err := rollbackConfig(&stdout, mockStore, "any-scope", 1)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
	if len(mockStore.saveData) > 0 {
		t.Error("no data should have been saved on failure")
	}
}

// TestRollbackConfig_Failure_SaveError tests the failure case when saving the rolled-back config fails.
func TestRollbackConfig_Failure_SaveError(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockRollbackSecureStore(map[string][]byte{
		"my-scope": []byte("some-data"),
	})
	mockStore.ForceSaveError = true
	var stdout bytes.Buffer

	// --- Execute ---
	err := rollbackConfig(&stdout, mockStore, "my-scope", 1)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}
