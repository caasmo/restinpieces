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

// MockSecureStore is a test-only implementation of the config.SecureStore.
type MockSecureStore struct {
	data          map[string][]byte
	format        string
	saveHistory   []string
	ForceGetError bool
}

// NewMockSecureStore creates a new mock store.
func NewMockSecureStore(initialData map[string][]byte) *MockSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockSecureStore{
		data:   initialData,
		format: "toml",
	}
}

// Get retrieves the configuration for a scope.
func (m *MockSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("%w: forced get error", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, m.format, nil
	}
	return data, m.format, nil
}

// Save updates the configuration for a scope.
func (m *MockSecureStore) Save(scope string, data []byte, format string, description string) error {
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

func TestAddOAuth2Provider_Success(t *testing.T) {
	t.Parallel()

	providerName := "github"
	mockStore := NewMockSecureStore(nil) // Empty store

	err := addOAuth2Provider(io.Discard, mockStore, providerName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify provider was added to config
	finalCfg := getConfigFromStore(t, mockStore)
	if _, exists := finalCfg.OAuth2Providers[providerName]; !exists {
		t.Error("provider was not added to config")
	}

	// Verify save description
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != "Added OAuth2 provider: github" {
		t.Error("incorrect save description")
	}
}

func getConfigFromStore(t *testing.T, store *MockSecureStore) *config.Config {
	t.Helper()
	data, _, err := store.Get(config.ScopeApplication, 0)
	if err != nil {
		t.Fatal(err)
	}

	var cfg config.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return &cfg
}

func TestAddOAuth2Provider_ExistingConfig(t *testing.T) {
	t.Parallel()

	initialCfg := config.Config{
		Server: config.Server{Addr: ":8080"},
		OAuth2Providers: map[string]config.OAuth2Provider{
			"github": {ClientID: "gh_client_id"},
		},
	}
	mockStore := NewMockSecureStoreWithConfig(t, initialCfg)

	err := addOAuth2Provider(io.Discard, mockStore, "google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	finalCfg := getConfigFromStore(t, mockStore)
	
	// Verify new provider was added
	if _, exists := finalCfg.OAuth2Providers["google"]; !exists {
		t.Error("new provider was not added")
	}
	
	// Verify existing provider unchanged
	if finalCfg.OAuth2Providers["github"].ClientID != "gh_client_id" {
		t.Error("existing provider was modified")
	}
	
	// Verify other config unchanged
	if finalCfg.Server.Addr != ":8080" {
		t.Error("server config was modified")
	}
}

func NewMockSecureStoreWithConfig(t *testing.T, cfg config.Config) *MockSecureStore {
	t.Helper()
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return NewMockSecureStore(map[string][]byte{
		config.ScopeApplication: data,
	})
}

func TestAddOAuth2Provider_Failure_ProviderAlreadyExists(t *testing.T) {
	// --- Setup ---
	providerName := "github"
	initialConfig := `
[oauth2_providers.github]
  ClientID = "gh_client_id"
`
	mockStore := NewMockSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(initialConfig),
	})
	var stdout bytes.Buffer

	// --- Execute ---
	err := addOAuth2Provider(&stdout, mockStore, providerName)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrProviderAlreadyExists) {
		t.Errorf("Expected error to be of type ErrProviderAlreadyExists, but got %T", err)
	}
}

func TestAddOAuth2Provider_Failure_MalformedInitialConfig(t *testing.T) {
	// --- Setup ---
	providerName := "gitlab"
	initialConfig := `[oauth2.providers`
	mockStore := NewMockSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(initialConfig),
	})
	var stdout bytes.Buffer

	// --- Execute ---
	err := addOAuth2Provider(&stdout, mockStore, providerName)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("Expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestAddOAuth2Provider_Failure_GetFromSecureStoreFails(t *testing.T) {
	// --- Setup ---
	providerName := "bitbucket"
	mockStore := NewMockSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout bytes.Buffer

	// --- Execute ---
	err := addOAuth2Provider(&stdout, mockStore, providerName)

	// --- Assert ---
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("Expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}
