package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// MockPathsSecureStore is a test-only implementation of the config.SecureStore.
type MockPathsSecureStore struct {
	data          map[string][]byte
	ForceGetError bool
}

// NewMockPathsSecureStore creates a new mock store.
func NewMockPathsSecureStore(initialData map[string][]byte) *MockPathsSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockPathsSecureStore{
		data: initialData,
	}
}

// Get retrieves the configuration for a scope.
func (m *MockPathsSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("%w: forced get error", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, "toml", nil
	}
	return data, "toml", nil
}

// Save is a no-op for paths tests.
func (m *MockPathsSecureStore) Save(scope string, data []byte, format string, description string) error {
	return nil
}

const sampleTomlConfig = `
[server]
addr = ":8080"
read_timeout = 30

[database]
host = "localhost"
port = 5432
`

func TestListPaths_Success(t *testing.T) {
	testCases := []struct {
		name           string
		filter         string
		expectedPaths  []string
		unexpectedPaths []string
	}{
		{
			name:           "No Filter",
			filter:         "",
			expectedPaths:  []string{"server.addr", "server.read_timeout", "database.host", "database.port"},
			unexpectedPaths: []string{},
		},
		{
			name:           "With Filter",
			filter:         "server",
			expectedPaths:  []string{"server.addr", "server.read_timeout"},
			unexpectedPaths: []string{"database.host", "database.port"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Setup ---
			mockStore := NewMockPathsSecureStore(map[string][]byte{
				config.ScopeApplication: []byte(sampleTomlConfig),
			})
			var stdout bytes.Buffer

			// --- Execute ---
			err := listPaths(&stdout, mockStore, config.ScopeApplication, tc.filter)

			// --- Assert ---
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := stdout.String()
			for _, path := range tc.expectedPaths {
				if !strings.Contains(output, path) {
					t.Errorf("expected output to contain path '%s', but it did not", path)
				}
			}
			for _, path := range tc.unexpectedPaths {
				if strings.Contains(output, path) {
					t.Errorf("expected output to not contain path '%s', but it did", path)
				}
			}
		})
	}
}

func TestListPaths_Success_NoPathsFound(t *testing.T) {
	testCases := []struct {
		name      string
		config    []byte
		filter    string
	}{
		{
			name:      "Empty Config",
			config:    []byte(""),
			filter:    "",
		},
		{
			name:      "Filter Matches Nothing",
			config:    []byte(sampleTomlConfig),
			filter:    "nonexistent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Setup ---
			mockStore := NewMockPathsSecureStore(map[string][]byte{
				config.ScopeApplication: tc.config,
			})

			// --- Execute ---
			err := listPaths(io.Discard, mockStore, config.ScopeApplication, tc.filter)

			// --- Assert ---
			if err != nil {
				t.Fatalf("expected no error, but got: %v", err)
			}
		})
	}
}

func TestListPaths_Failure_SecureStoreError(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockPathsSecureStore(nil)
	mockStore.ForceGetError = true

	// --- Execute ---
	err := listPaths(io.Discard, mockStore, config.ScopeApplication, "")

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestListPaths_Failure_MalformedToml(t *testing.T) {
	// --- Setup ---
	malformedConfig := `[server` // Intentionally broken TOML
	mockStore := NewMockPathsSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(malformedConfig),
	})

	// --- Execute ---
	err := listPaths(io.Discard, mockStore, config.ScopeApplication, "")

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrTomlLoad) {
		t.Errorf("expected error to wrap ErrTomlLoad, got %v", err)
	}
}
