package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// This MockSecureStore is a simplified version for dump command tests.
// A shared test utility could be a future improvement.
type MockDumpSecureStore struct {
	data          map[string][]byte
	ForceGetError bool
}

func NewMockDumpSecureStore(initialData map[string][]byte) *MockDumpSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockDumpSecureStore{
		data: initialData,
	}
}

func (m *MockDumpSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, "toml", nil
	}
	return data, "toml", nil
}

// Save is a no-op for dump tests but required to satisfy the SecureStore interface.
func (m *MockDumpSecureStore) Save(scope string, data []byte, format string, description string) error {
	return nil
}

// TestDumpConfig_Success verifies successful writing of config data.
func TestDumpConfig_Success(t *testing.T) {
	scope := "test_app"
	expectedOutput := "config_data_for_test_app"
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		scope: []byte(expectedOutput),
	})
	var stdout bytes.Buffer

	err := dumpConfig(&stdout, mockStore, scope)

	if err != nil {
		t.Fatalf("dumpConfig() returned an unexpected error: %v", err)
	}
	if got := stdout.String(); got != expectedOutput {
		t.Errorf("dumpConfig() output = %q, want %q", got, expectedOutput)
	}
}

// TestDumpConfig_DefaultScope verifies use of the default application scope.
func TestDumpConfig_DefaultScope(t *testing.T) {
	expectedOutput := "application_specific_data"
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(expectedOutput),
	})
	var stdout bytes.Buffer

	err := dumpConfig(&stdout, mockStore, "") // Empty scope triggers default

	if err != nil {
		t.Fatalf("dumpConfig() with empty scope returned an unexpected error: %v", err)
	}
	if got := stdout.String(); got != expectedOutput {
		t.Errorf("dumpConfig() output = %q, want %q", got, expectedOutput)
	}
}

// TestDumpConfig_Failure_StoreReadError tests failure on store read error.
func TestDumpConfig_Failure_StoreReadError(t *testing.T) {
	mockStore := NewMockDumpSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout bytes.Buffer

	err := dumpConfig(&stdout, mockStore, "any_scope")

	if err == nil {
		t.Fatal("dumpConfig() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("dumpConfig() error = %v, want error wrapping %v", err, ErrSecureStoreGet)
	}
}

// failingWriter is an io.Writer that always returns an error.
type failingWriter struct{}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("forced write error")
}

// TestDumpConfig_Failure_OutputWriteError tests failure on output write error.
func TestDumpConfig_Failure_OutputWriteError(t *testing.T) {
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		"any_scope": []byte("some_data"),
	})
	var failingStdout failingWriter

	err := dumpConfig(&failingStdout, mockStore, "any_scope")

	if err == nil {
		t.Fatal("dumpConfig() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrWriteOutput) {
		t.Errorf("dumpConfig() error = %v, want error wrapping %v", err, ErrWriteOutput)
	}
}
