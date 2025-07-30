package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// MockRmOAuth2SecureStore is a test-only implementation of the config.SecureStore.
// It is adapted from the one in add_oauth2_command_test.go.
type MockRmOAuth2SecureStore struct {
	data          map[string][]byte
	format        string
	saveHistory   []string
	ForceGetError bool
	ForceSaveError bool
}

// NewMockRmOAuth2SecureStore creates a new mock store.
func NewMockRmOAuth2SecureStore(initialData map[string][]byte) *MockRmOAuth2SecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockRmOAuth2SecureStore{
		data:   initialData,
		format: "toml",
	}
}

// Get retrieves the configuration for a scope.
func (m *MockRmOAuth2SecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("%w: forced get error", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		// Return empty data if scope doesn't exist, simulating an empty config
		return []byte{}, m.format, nil
	}
	return data, m.format, nil
}

// Save updates the configuration for a scope.
func (m *MockRmOAuth2SecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("%w: forced save error", ErrSecureStoreSave)
	}
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

// Helper to create a mock store initialized with a specific config struct.
func newMockStoreWithConfig(t *testing.T, cfg config.Config) *MockRmOAuth2SecureStore {
	t.Helper()
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config for mock store: %v", err)
	}
	return NewMockRmOAuth2SecureStore(map[string][]byte{
		config.ScopeApplication: data,
	})
}

// Helper to unmarshal the config from the mock store for verification.
func getConfigFromRmStore(t *testing.T, store *MockRmOAuth2SecureStore) *config.Config {
	t.Helper()
	data, _, err := store.Get(config.ScopeApplication, 0)
	if err != nil {
		t.Fatalf("failed to get config from mock store: %v", err)
	}

	var cfg config.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal config from store data: %v", err)
	}
	return &cfg
}

func TestRemoveOAuth2Provider_Success(t *testing.T) {
	// --- Setup ---
	providerToRemove := "github"
	initialCfg := config.Config{
		Server: config.Server{Addr: ":8080"},
		OAuth2Providers: map[string]config.OAuth2Provider{
			providerToRemove: {ClientID: "gh_id"},
			"google":         {ClientID: "google_id"},
		},
	}
	mockStore := newMockStoreWithConfig(t, initialCfg)
	var stdout bytes.Buffer

	// --- Execute ---
	err := removeOAuth2Provider(&stdout, mockStore, providerToRemove)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify provider was removed
	finalCfg := getConfigFromRmStore(t, mockStore)
	if _, exists := finalCfg.OAuth2Providers[providerToRemove]; exists {
		t.Errorf("provider '%s' was not removed from config", providerToRemove)
	}

	// Verify other provider and config still exist
	if _, exists := finalCfg.OAuth2Providers["google"]; !exists {
		t.Error("provider 'google' was unexpectedly removed")
	}
	if finalCfg.Server.Addr != ":8080" {
		t.Error("other config sections were unexpectedly modified")
	}

	// Verify save description
	expectedDesc := fmt.Sprintf("Removed OAuth2 provider: %s", providerToRemove)
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != expectedDesc {
		t.Errorf("incorrect save description: got %q, want %q", mockStore.saveHistory[0], expectedDesc)
	}

	// Verify output
	expectedOutput := fmt.Sprintf("Successfully removed OAuth2 provider '%s'\n", providerToRemove)
	if stdout.String() != expectedOutput {
		t.Errorf("unexpected output: got %q, want %q", stdout.String(), expectedOutput)
	}
}

func TestRemoveOAuth2Provider_Failure_ProviderNotFound(t *testing.T) {
	// --- Setup ---
	providerToRemove := "gitlab" // This provider does not exist
	initialCfg := config.Config{
		OAuth2Providers: map[string]config.OAuth2Provider{
			"github": {ClientID: "gh_id"},
		},
	}
	mockStore := newMockStoreWithConfig(t, initialCfg)

	// --- Execute ---
	err := removeOAuth2Provider(io.Discard, mockStore, providerToRemove)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("Expected error to wrap ErrProviderNotFound, got %v", err)
	}
}

func TestRemoveOAuth2Provider_Failure_GetError(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockRmOAuth2SecureStore(nil)
	mockStore.ForceGetError = true

	// --- Execute ---
	err := removeOAuth2Provider(io.Discard, mockStore, "any-provider")

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("Expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestRemoveOAuth2Provider_Failure_SaveError(t *testing.T) {
	// --- Setup ---
	initialCfg := config.Config{
		OAuth2Providers: map[string]config.OAuth2Provider{"github": {}},
	}
	mockStore := newMockStoreWithConfig(t, initialCfg)
	mockStore.ForceSaveError = true

	// --- Execute ---
	err := removeOAuth2Provider(io.Discard, mockStore, "github")

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("Expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

func TestRemoveOAuth2Provider_Failure_UnmarshalError(t *testing.T) {
	// --- Setup ---
	malformedToml := []byte(`[oauth2_providers.github`) // Invalid TOML
	mockStore := NewMockRmOAuth2SecureStore(map[string][]byte{
		config.ScopeApplication: malformedToml,
	})

	// --- Execute ---
	err := removeOAuth2Provider(io.Discard, mockStore, "github")

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("Expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}
