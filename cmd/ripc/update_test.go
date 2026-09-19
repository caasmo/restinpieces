package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	toml "github.com/pelletier/go-toml"
)

// MockUpdateSecureStore is a test-only implementation of config.SecureStore for update command tests.
type MockUpdateSecureStore struct {
	data           map[string][]byte
	format         string
	saveHistory    []string
	ForceGetError  bool
	ForceSaveError bool
}

func NewMockUpdateSecureStore(initialData map[string][]byte) *MockUpdateSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockUpdateSecureStore{
		data:   initialData,
		format: "toml",
	}
}

func (m *MockUpdateSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, m.format, nil
	}
	return data, m.format, nil
}

func (m *MockUpdateSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("forced save error: %w", ErrSecureStoreSave)
	}
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

const updateTestConf = `
[server]
  addr = ":8080"

[jwt]
  auth_secret = "old"
  password_reset_secret = "old"
  email_change_otp_secret = "old"
  verification_email_otp_secret = "old"
  oauth2_state_secret = "old"
`

func isAlphanumeric(s string) bool {
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func getUpdateTreeFromStore(t *testing.T, store *MockUpdateSecureStore, scope string) *toml.Tree {
	t.Helper()
	data, _, err := store.Get(scope, 0)
	if err != nil {
		t.Fatalf("failed to get data from mock store: %v", err)
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		t.Fatalf("failed to load toml from store data: %v", err)
	}
	return tree
}

func TestUpdate_SingleKey(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "jwt.auth_secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockStore.saveHistory) != 1 {
		t.Fatalf("expected 1 save, got %d", len(mockStore.saveHistory))
	}

	tree := getUpdateTreeFromStore(t, mockStore, scope)
	fresh := tree.Get("jwt.auth_secret")
	freshStr, ok := fresh.(string)
	if !ok {
		t.Fatalf("expected jwt.auth_secret to be string, got %T", fresh)
	}
	if len(freshStr) != 32 {
		t.Errorf("expected fresh secret length 32, got %d", len(freshStr))
	}
	if !isAlphanumeric(freshStr) {
		t.Errorf("expected fresh secret to be alphanumeric, got %q", freshStr)
	}
	if freshStr == "old" {
		t.Errorf("expected fresh secret to differ from old value")
	}

	untouched := []string{
		"jwt.password_reset_secret",
		"jwt.email_change_otp_secret",
		"jwt.verification_email_otp_secret",
		"jwt.oauth2_state_secret",
	}
	for _, path := range untouched {
		if got := tree.Get(path); got != "old" {
			t.Errorf("expected %s to stay 'old', got %v", path, got)
		}
	}
}

func TestUpdate_JwtLabel(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "jwt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockStore.saveHistory) != 1 {
		t.Fatalf("expected 1 save, got %d", len(mockStore.saveHistory))
	}

	expectedDesc := "Updated 'jwt.auth_secret, jwt.email_change_otp_secret, jwt.oauth2_state_secret, jwt.password_reset_secret, jwt.verification_email_otp_secret'"
	if mockStore.saveHistory[0] != expectedDesc {
		t.Errorf("expected save description %q, got %q", expectedDesc, mockStore.saveHistory[0])
	}

	tree := getUpdateTreeFromStore(t, mockStore, scope)
	for _, path := range []string{
		"jwt.auth_secret",
		"jwt.password_reset_secret",
		"jwt.email_change_otp_secret",
		"jwt.verification_email_otp_secret",
		"jwt.oauth2_state_secret",
	} {
		got, ok := tree.Get(path).(string)
		if !ok {
			t.Errorf("expected %s to be string, got %T", path, tree.Get(path))
			continue
		}
		if len(got) != 32 || !isAlphanumeric(got) {
			t.Errorf("expected %s to be 32-char alphanumeric, got %q", path, got)
		}
		if got == "old" {
			t.Errorf("expected %s to be regenerated, still 'old'", path)
		}
	}
}

func TestUpdate_CustomDesc(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	description := "rotate after incident"

	err := updateValues(ui, mockStore, scope, description, "jwt.auth_secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockStore.saveHistory) != 1 || mockStore.saveHistory[0] != description {
		t.Errorf("expected save history to contain %q, got %v", description, mockStore.saveHistory)
	}
}

func TestUpdate_Failure_MissingPath(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected error to wrap ErrPathNotFound, got %v", err)
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save on updater error, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_Failure_MalformedTOML(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte("[jwt")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "jwt")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestUpdate_UnknownLabel(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "server.port")
	if !errors.Is(err, ErrUnknownUpdateLabel) {
		t.Fatalf("expected error to wrap ErrUnknownUpdateLabel, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_Failure_StoreReadError(t *testing.T) {
	mockStore := NewMockUpdateSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, "app", "", "jwt")
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestUpdate_Failure_StoreSaveError(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTestConf)})
	mockStore.ForceSaveError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "jwt")
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

func TestHandleUpdateCommand_Help(t *testing.T) {
	mockStore := NewMockUpdateSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleUpdateCommand(mockStore, []string{"-h"}, ui)
	if err != nil {
		t.Fatalf("expected no error for -h, got %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Usage:")) {
		t.Errorf("expected usage on stdout, got: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %q", stderr.String())
	}
}

func TestUpdateGroups_ContainsUserAgent(t *testing.T) {
	group, ok := updateGroups["block_user_agent"]
	if !ok {
		t.Fatal(`expected updateGroups to contain "block_user_agent"`)
	}
	if group.updater == nil {
		t.Error("expected block_user_agent to have an updater")
	}
}

func TestUpdate_RefusesPartialNames(t *testing.T) {
	for _, arg := range []string{"s", "se", "ss", ""} {
		mockStore := NewMockUpdateSecureStore(nil)
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := updateValues(ui, mockStore, "app", "", arg)
		if !errors.Is(err, ErrUnknownUpdateLabel) {
			t.Errorf("updateValues(%q) expected ErrUnknownUpdateLabel, got %v", arg, err)
		}
	}
}
