package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// DiffMockSecureStore is a test-only implementation of the config.SecureStore,
// specifically for testing the diff command.
type DiffMockSecureStore struct {
	// data maps scope -> generation -> config data
	data            map[string]map[int][]byte
	forceGetErrorOn map[int]bool // Key is generation number
}

// NewDiffMockSecureStore creates a new mock store for diffing.
func NewDiffMockSecureStore() *DiffMockSecureStore {
	return &DiffMockSecureStore{
		data:            make(map[string]map[int][]byte),
		forceGetErrorOn: make(map[int]bool),
	}
}

// AddConfig adds a configuration version to the mock store.
func (m *DiffMockSecureStore) AddConfig(scope string, generation int, configData []byte) {
	if _, ok := m.data[scope]; !ok {
		m.data[scope] = make(map[int][]byte)
	}
	m.data[scope][generation] = configData
}

// Get retrieves the configuration for a scope and generation.
func (m *DiffMockSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.forceGetErrorOn[generation] {
		return nil, "", fmt.Errorf("%w: forced get error for generation %d", ErrSecureStoreGet, generation)
	}
	if scopeData, ok := m.data[scope]; ok {
		if configData, ok := scopeData[generation]; ok {
			return configData, "toml", nil
		}
	}
	return []byte{}, "toml", nil
}

// Save is not used for diff tests, so it's a no-op.
func (m *DiffMockSecureStore) Save(scope string, data []byte, format string, description string) error {
	return nil
}

// TestDiffConfig_WithDifferences verifies that a correct diff is generated when
// the latest config has changed from a previous generation.
func TestDiffConfig_WithDifferences(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 1, []byte(`
[server]
addr = ":8080"
`))
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`
[server]
addr = ":9090"
`))
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	// The TOML library normalizes quotes, so we check for single quotes.
	if !strings.Contains(output, "-addr = ':8080'") {
		t.Errorf("output does not contain removed line. Got:\n%s", output)
	}
	if !strings.Contains(output, "+addr = ':9090'") {
		t.Errorf("output does not contain added line. Got:\n%s", output)
	}
}

// TestDiffConfig_NoDifferences ensures the correct message is printed when there
// are no changes between two config versions.
func TestDiffConfig_NoDifferences(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	configData := []byte(`
[server]
addr = ":8080"
`)
	mockStore.AddConfig(config.ScopeApplication, 1, configData)
	mockStore.AddConfig(config.ScopeApplication, 0, configData)
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout.String(), "No differences") {
		t.Errorf("expected 'No differences' message, got: %s", stdout.String())
	}
}

// TestDiffConfig_NoSemanticDifferences verifies that the diff ignores cosmetic
// differences (like key order or comments) and only reports semantic changes.
func TestDiffConfig_NoSemanticDifferences(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 1, []byte(`
version = "1.0"
name = "app"
`))
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`
name = "app"
version = "1.0"
`))
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout.String(), "No differences") {
		t.Errorf("expected 'No differences' message, got: %s", stdout.String())
	}
}

// TestDiffConfig_Failure_GetLatestFails tests the error handling when the
// secureStore fails to retrieve the latest configuration.
func TestDiffConfig_Failure_GetLatestFails(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.forceGetErrorOn[0] = true // Fail on getting 'latest'
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("Expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

// TestDiffConfig_Failure_GetTargetFails tests error handling when the secureStore
// fails to retrieve the target generation's configuration.
func TestDiffConfig_Failure_GetTargetFails(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`[server]`))
	mockStore.forceGetErrorOn[1] = true // Fail on getting target generation
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("Expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

// TestDiffConfig_Failure_MalformedLatestConfig tests robustness against corrupted
// data in the store for the latest configuration.
func TestDiffConfig_Failure_MalformedLatestConfig(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`[server`))
	mockStore.AddConfig(config.ScopeApplication, 1, []byte(`[server]`))
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("Expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

// TestDiffConfig_Failure_MalformedTargetConfig tests robustness against corrupted
// data in the store for the older generation.
func TestDiffConfig_Failure_MalformedTargetConfig(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`[server]`))
	mockStore.AddConfig(config.ScopeApplication, 1, []byte(`[server`))
	var stdout bytes.Buffer

	// --- Execute ---
	err := diffConfig(&stdout, mockStore, config.ScopeApplication, 1)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("Expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}
