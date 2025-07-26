package main

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
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

func TestAddOAuth2Provider_Success_AddNewProviderToEmptyConfig(t *testing.T) {
	// --- Setup ---
	providerName := "github"
	initialConfig := `# Empty config`
	mockStore := NewMockSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(initialConfig),
	})
	var stdout bytes.Buffer

	// --- Execute ---
	err := addOAuth2Provider(&stdout, mockStore, providerName)

	// --- Assert ---
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
	expectedStdout := "Successfully added OAuth2 provider 'github'\nPlease configure the provider's URLs, scopes and credentials\n"
	if stdout.String() != expectedStdout {
		t.Errorf("Expected stdout to be '%s', but got '%s'", expectedStdout, stdout.String())
	}

	// Verify final config
	var finalCfg config.Config
	finalData := mockStore.data[config.ScopeApplication]
	if err := toml.Unmarshal(finalData, &finalCfg); err != nil {
		t.Fatalf("Failed to unmarshal final config: %v", err)
	}

	expectedProvider := config.OAuth2Provider{
		Name:            "github",
		DisplayName:     "Github",
		RedirectURLPath: "/oauth2/github/callback",
		PKCE:            true,
		Scopes:          []string{},
	}
	if !reflect.DeepEqual(expectedProvider, finalCfg.OAuth2Providers["github"]) {
		t.Errorf("Expected provider config %+v, got %+v", expectedProvider, finalCfg.OAuth2Providers["github"])
	}
	if mockStore.saveHistory[0] != "Added OAuth2 provider: github" {
		t.Errorf("Expected save message 'Added OAuth2 provider: github', got '%s'", mockStore.saveHistory[0])
	}
}

func TestAddOAuth2Provider_Success_AddNewProviderToExistingConfig(t *testing.T) {
	// --- Setup ---
	providerName := "google"
	initialConfig := `
[server]
  addr = ":8080"
[oauth2_providers.github]
  ClientID = "gh_client_id"
  scopes = []
`
	mockStore := NewMockSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(initialConfig),
	})
	var stdout bytes.Buffer

	// --- Execute ---
	err := addOAuth2Provider(&stdout, mockStore, providerName)

	// --- Assert ---
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
	expectedStdout := "Successfully added OAuth2 provider 'google'\nPlease configure the provider's URLs, scopes and credentials\n"
	if stdout.String() != expectedStdout {
		t.Errorf("Expected stdout to be '%s', but got '%s'", expectedStdout, stdout.String())
	}

	// Verify final config
	var finalCfg config.Config
	finalData := mockStore.data[config.ScopeApplication]
	if err := toml.Unmarshal(finalData, &finalCfg); err != nil {
		t.Fatalf("Failed to unmarshal final config: %v", err)
	}

	// Check that new provider was added correctly
	expectedGoogleProvider := config.OAuth2Provider{
		Name:			"google",
		DisplayName:	"Google",
		RedirectURLPath: "/oauth2/google/callback",
		PKCE:			true,
		Scopes:			[]string{},
	}
	if !reflect.DeepEqual(expectedGoogleProvider, finalCfg.OAuth2Providers["google"]) {
		t.Errorf("Expected google provider config %+v, got %+v", expectedGoogleProvider, finalCfg.OAuth2Providers["google"])
	}

	// Check that existing provider is untouched
	var initialCfg config.Config
	if err := toml.Unmarshal([]byte(initialConfig), &initialCfg); err != nil {
		t.Fatalf("Failed to unmarshal initial config: %v", err)
	}
	if !reflect.DeepEqual(initialCfg.OAuth2Providers["github"], finalCfg.OAuth2Providers["github"]) {
		t.Errorf("Expected github provider config %+v, got %+v", initialCfg.OAuth2Providers["github"], finalCfg.OAuth2Providers["github"])
	}
	if finalCfg.Server.Addr != ":8080" {
		t.Errorf("Expected server address ':8080', got '%s'", finalCfg.Server.Addr)
	}
	if mockStore.saveHistory[0] != "Added OAuth2 provider: google" {
		t.Errorf("Expected save message 'Added OAuth2 provider: google', got '%s'", mockStore.saveHistory[0])
	}
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
