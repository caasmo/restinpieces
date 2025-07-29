package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// MockSaveSecureStore is a test-only implementation of the config.SecureStore
// tailored for testing the save command. It only implements the Save method.
type MockSaveSecureStore struct {
	data           map[string][]byte
	format         string
	saveHistory    []string
	ForceSaveError bool
}

// NewMockSaveSecureStore creates a new mock store.
func NewMockSaveSecureStore() *MockSaveSecureStore {
	return &MockSaveSecureStore{
		data: make(map[string][]byte),
	}
}

// Get is not implemented and will panic if called.
func (m *MockSaveSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	panic("not implemented")
}

// Save updates the configuration for a scope.
func (m *MockSaveSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("%w: forced save error", ErrSecureStoreSave)
	}
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

// TestSaveConfigFromData tests the core logic of the save command.
func TestSaveConfigFromData(t *testing.T) {
	testCases := []struct {
		name          string
		scopeIn       string
		expectedScope string
	}{
		{
			name:          "DefaultScope",
			scopeIn:       "",
			expectedScope: config.ScopeApplication,
		},
		{
			name:          "ExplicitScope",
			scopeIn:       "my-custom-scope",
			expectedScope: "my-custom-scope",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Setup ---
			mockStore := NewMockSaveSecureStore()
			var stdout bytes.Buffer
			filename := "test.toml"
			data := []byte("[server]\naddr = \":8080\"")

			// --- Execute ---
			err := saveConfigFromData(&stdout, mockStore, tc.scopeIn, filename, data)

			// --- Assert ---
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify data in store
			savedData, ok := mockStore.data[tc.expectedScope]
			if !ok {
				t.Fatalf("scope '%s' not found in store", tc.expectedScope)
			}
			if !bytes.Equal(savedData, data) {
				t.Errorf("data mismatch: got %q, want %q", string(savedData), string(data))
			}

			// Verify save description
			expectedDesc := fmt.Sprintf("Inserted from file: %s", filepath.Base(filename))
			if len(mockStore.saveHistory) != 1 || mockStore.saveHistory[0] != expectedDesc {
				t.Errorf("description mismatch: got %q, want %q", mockStore.saveHistory[0], expectedDesc)
			}

			// Verify stdout message
			expectedOut := fmt.Sprintf("Successfully saved file '%s' to scope '%s' in database\n", filename, tc.expectedScope)
			if stdout.String() != expectedOut {
				t.Errorf("stdout mismatch: got %q, want %q", stdout.String(), expectedOut)
			}
		})
	}
}

// TestSaveConfig_Failure_SaveError tests when the secure store fails to save.
func TestSaveConfig_Failure_SaveError(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockSaveSecureStore()
	mockStore.ForceSaveError = true
	var stdout bytes.Buffer

	// --- Execute ---
	err := saveConfigFromData(&stdout, mockStore, "scope", "file.toml", []byte("data"))

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
	if len(mockStore.data) > 0 {
		t.Error("data should not have been saved to the store on failure")
	}
}
