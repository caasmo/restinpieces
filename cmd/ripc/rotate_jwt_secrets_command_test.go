package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// MockRotateSecureStore is a test-only implementation of config.SecureStore
// tailored for testing the rotate-jwt-secrets command.
type MockRotateSecureStore struct {
	getData        []byte
	getFormat      string
	saveData       []byte
	saveFormat     string
	saveHistory    []string
	ForceGetError  bool
	ForceSaveError bool
}

// NewMockRotateSecureStore creates a new mock store.
func NewMockRotateSecureStore(initialData []byte) *MockRotateSecureStore {
	return &MockRotateSecureStore{
		getData:   initialData,
		getFormat: "toml",
	}
}

// Get retrieves the configuration.
func (m *MockRotateSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("%w: forced get error", ErrSecureStoreGet)
	}
	return m.getData, m.getFormat, nil
}

// Save updates the configuration.
func (m *MockRotateSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("%w: forced save error", ErrSecureStoreSave)
	}
	m.saveData = data
	m.saveFormat = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

// TestRotateJwtSecrets_Success tests the successful rotation of JWT secrets.
func TestRotateJwtSecrets_Success(t *testing.T) {
	// --- Setup ---
	initialCfg := config.Config{
		Server: config.Server{Addr: ":8080"},
		Jwt: config.Jwt{
			AuthSecret:              "initial_auth_secret",
			VerificationEmailSecret: "initial_verification_secret",
			PasswordResetSecret:     "initial_password_reset_secret",
			EmailChangeSecret:       "initial_email_change_secret",
		},
	}
	initialToml, err := toml.Marshal(initialCfg)
	if err != nil {
		t.Fatalf("failed to marshal initial config: %v", err)
	}

	mockStore := NewMockRotateSecureStore(initialToml)
	var stdout bytes.Buffer

	// --- Execute ---
	err = rotateJwtSecrets(&stdout, mockStore)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify secrets were changed
	var finalCfg config.Config
	if err := toml.Unmarshal(mockStore.saveData, &finalCfg); err != nil {
		t.Fatalf("failed to unmarshal final config: %v", err)
	}

	if finalCfg.Jwt.AuthSecret == initialCfg.Jwt.AuthSecret {
		t.Error("AuthSecret was not rotated")
	}
	if finalCfg.Jwt.VerificationEmailSecret == initialCfg.Jwt.VerificationEmailSecret {
		t.Error("VerificationEmailSecret was not rotated")
	}
	if finalCfg.Jwt.PasswordResetSecret == initialCfg.Jwt.PasswordResetSecret {
		t.Error("PasswordResetSecret was not rotated")
	}
	if finalCfg.Jwt.EmailChangeSecret == initialCfg.Jwt.EmailChangeSecret {
		t.Error("EmailChangeSecret was not rotated")
	}

	// Verify other config is untouched
	if finalCfg.Server.Addr != initialCfg.Server.Addr {
		t.Errorf("server address was modified: got %q, want %q", finalCfg.Server.Addr, initialCfg.Server.Addr)
	}

	// Verify save description
	if len(mockStore.saveHistory) != 1 || mockStore.saveHistory[0] != "Renewed all JWT secrets" {
		t.Error("incorrect save description")
	}

	// Verify stdout message
	expectedOut := "Successfully renewed all JWT secrets for application scope\n"
	if stdout.String() != expectedOut {
		t.Errorf("stdout mismatch: got %q, want %q", stdout.String(), expectedOut)
	}
}

// TestRotateJwtSecrets_Failure_GetError tests failure when retrieving the config fails.
func TestRotateJwtSecrets_Failure_GetError(t *testing.T) {
	// --- Setup ---
	mockStore := NewMockRotateSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout bytes.Buffer

	// --- Execute ---
	err := rotateJwtSecrets(&stdout, mockStore)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
	if mockStore.saveData != nil {
		t.Error("no data should have been saved on failure")
	}
}

// TestRotateJwtSecrets_Failure_UnmarshalError tests failure with malformed config.
func TestRotateJwtSecrets_Failure_UnmarshalError(t *testing.T) {
	// --- Setup ---
	malformedToml := []byte(`[jwt] auth_secret = "abc"a`)
	mockStore := NewMockRotateSecureStore(malformedToml)
	var stdout bytes.Buffer

	// --- Execute ---
	err := rotateJwtSecrets(&stdout, mockStore)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
	if mockStore.saveData != nil {
		t.Error("no data should have been saved on failure")
	}
}

// TestRotateJwtSecrets_Failure_SaveError tests failure when saving the config fails.
func TestRotateJwtSecrets_Failure_SaveError(t *testing.T) {
	// --- Setup ---
	initialToml, _ := toml.Marshal(config.Config{})
	mockStore := NewMockRotateSecureStore(initialToml)
	mockStore.ForceSaveError = true
	var stdout bytes.Buffer

	// --- Execute ---
	err := rotateJwtSecrets(&stdout, mockStore)

	// --- Assert ---
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}
