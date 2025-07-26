package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Using the same mock store from dump_command_test.go for simplicity.
// A shared test utility could be a future improvement.
type MockGetSecureStore struct {
	data          map[string][]byte
	ForceGetError bool
}

func NewMockGetSecureStore(initialData map[string][]byte) *MockGetSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockGetSecureStore{
		data: initialData,
	}
}

func (m *MockGetSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, "toml", nil
	}
	return data, "toml", nil
}

func (m *MockGetSecureStore) Save(scope string, data []byte, format string, description string) error {
	return nil // Not needed for get command tests
}

const conf = `
public_dir = "/var/www/public"

[server]
  addr = ":8080"
  enable_tls = true
  read_timeout = "5s"

[log]
  level = "info"
  [log.batch]
    flush_size = 200
    db_path = "/var/log/app.db"

[oauth2_providers.github]
  name = "github"
  display_name = "GitHub"
  pkce = true
`

// TestGetAndPrintConfigPaths_Success_NoFilter verifies all paths are printed without a filter.
func TestGetAndPrintConfigPaths_Success_NoFilter(t *testing.T) {
	scope := "app"
	mockStore := NewMockGetSecureStore(map[string][]byte{
		scope: []byte(conf),
	})
	var stdout bytes.Buffer

	err := getAndPrintConfigPaths(&stdout, mockStore, scope, "")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	output := stdout.String()
	expectedSubstrings := []string{
		`log.batch.db_path = /var/log/app.db`,
		`log.batch.flush_size = 200`,
		`log.level = info`,
		`oauth2_providers.github.display_name = GitHub`,
		`oauth2_providers.github.name = github`,
		`oauth2_providers.github.pkce = true`,
		`public_dir = /var/www/public`,
		`server.addr = :8080`,
		`server.enable_tls = true`,
		`server.read_timeout = 5s`,
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("Expected output to contain %q, but it did not.\nFull output:\n%s", sub, output)
		}
	}
}

// TestGetAndPrintConfigPaths_Success_WithFilter verifies that filtering works correctly.
func TestGetAndPrintConfigPaths_Success_WithFilter(t *testing.T) {
	scope := "app"
	mockStore := NewMockGetSecureStore(map[string][]byte{
		scope: []byte(conf),
	})
	var stdout bytes.Buffer

	err := getAndPrintConfigPaths(&stdout, mockStore, scope, "server")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	output := stdout.String()
	expectedSubstrings := []string{
		`server.addr = :8080`,
		`server.enable_tls = true`,
		`server.read_timeout = 5s`,
	}
	unexpectedSubstrings := []string{
		"log",
		"oauth2_providers",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("Expected output to contain %q, but it did not", sub)
		}
	}
	for _, sub := range unexpectedSubstrings {
		if strings.Contains(output, sub) {
			t.Errorf("Expected output to NOT contain %q, but it did", sub)
		}
	}
}

// TestGetAndPrintConfigPaths_NoResults_WithFilter verifies the message for a non-matching filter.
func TestGetAndPrintConfigPaths_NoResults_WithFilter(t *testing.T) {
	scope := "app"
	mockStore := NewMockGetSecureStore(map[string][]byte{
		scope: []byte(conf),
	})
	var stdout bytes.Buffer

	err := getAndPrintConfigPaths(&stdout, mockStore, scope, "nonexistent")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedOutput := "No TOML paths with values matching 'nonexistent' found in scope 'app'.\n"
	if stdout.String() != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, stdout.String())
	}
}

// TestGetAndPrintConfigPaths_EmptyConfig verifies the message for an empty configuration.
func TestGetAndPrintConfigPaths_EmptyConfig(t *testing.T) {
	scope := "empty_scope"
	mockStore := NewMockGetSecureStore(map[string][]byte{
		scope: []byte(""), // Empty config
	})
	var stdout bytes.Buffer

	err := getAndPrintConfigPaths(&stdout, mockStore, scope, "")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedOutput := "No TOML paths with values found in configuration for scope 'empty_scope'.\n"
	if stdout.String() != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, stdout.String())
	}
}

// TestGetAndPrintConfigPaths_Failure_StoreReadError tests failure on store read error.
func TestGetAndPrintConfigPaths_Failure_StoreReadError(t *testing.T) {
	mockStore := NewMockGetSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout bytes.Buffer

	err := getAndPrintConfigPaths(&stdout, mockStore, "any_scope", "")

	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("Expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

// TestGetAndPrintConfigPaths_Failure_MalformedTOML tests failure on malformed TOML data.
func TestGetAndPrintConfigPaths_Failure_MalformedTOML(t *testing.T) {
	scope := "malformed"
	mockStore := NewMockGetSecureStore(map[string][]byte{
		scope: []byte(`[server`),
	})
	var stdout bytes.Buffer

	err := getAndPrintConfigPaths(&stdout, mockStore, scope, "")

	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("Expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}
