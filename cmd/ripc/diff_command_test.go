package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// DiffMockSecureStore is a test-only implementation of the config.SecureStore,
// specifically for testing the diff command.
type DiffMockSecureStore struct {
	// data maps scope -> generation -> config data
	data             map[string]map[int][]byte
	forceGetErrorOn  map[int]bool // Key is generation number
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
	// Return empty data if not found, which is a valid scenario.
	return []byte{}, "toml", nil
}

// Save is not used for diff tests, so it's a no-op.
func (m *DiffMockSecureStore) Save(scope string, data []byte, format string, description string) error {
	return nil
}

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

func TestDiffConfig_Failure_GetTargetFails(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`[server]`)) // Latest is fine
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

func TestDiffConfig_Failure_MalformedLatestConfig(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`[server`))   // Malformed latest
	mockStore.AddConfig(config.ScopeApplication, 1, []byte(`[server]`)) // Valid target
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

func TestDiffConfig_Failure_MalformedTargetConfig(t *testing.T) {
	// --- Setup ---
	mockStore := NewDiffMockSecureStore()
	mockStore.AddConfig(config.ScopeApplication, 0, []byte(`[server]`))   // Valid latest
	mockStore.AddConfig(config.ScopeApplication, 1, []byte(`[server`)) // Malformed target
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