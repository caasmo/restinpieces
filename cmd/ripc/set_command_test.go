package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/pelletier/go-toml"
)

// MockSetSecureStore is a test-only implementation of config.SecureStore for set command tests.
type MockSetSecureStore struct {
	data          map[string][]byte
	format        string
	saveHistory   []string
	ForceGetError bool
	ForceSaveError bool
}

func NewMockSetSecureStore(initialData map[string][]byte) *MockSetSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockSetSecureStore{
		data:   initialData,
		format: "toml",
	}
}

func (m *MockSetSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, m.format, nil
	}
	return data, m.format, nil
}

func (m *MockSetSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("forced save error: %w", ErrSecureStoreSave)
	}
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

const setTestConf = `
public_dir = "/var/www/public"
[server]
  addr = ":8080"
  enable_tls = true
[log.batch]
  flush_size = 200
`

func getTreeFromStore(t *testing.T, store *MockSetSecureStore, scope string) *toml.Tree {
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

func TestSetConfigValue_Success_String(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout bytes.Buffer
	path := "server.addr"
	value := `"localhost:9999"` // TOML strings need quotes

	err := setConfigValue(&stdout, mockStore, scope, "toml", "", path, value)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get(path); got != "localhost:9999" {
		t.Errorf("expected %s to be 'localhost:9999', got %v", path, got)
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != "Updated field 'server.addr'" {
		t.Error("incorrect default save description")
	}
}

func TestSetConfigValue_Success_Numeric(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout bytes.Buffer
	path := "log.batch.flush_size"
	value := "500"

	err := setConfigValue(&stdout, mockStore, scope, "toml", "", path, value)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get(path); got.(int64) != 500 {
		t.Errorf("expected %s to be 500, got %v (%T)", path, got, got)
	}
}

func TestSetConfigValue_Success_Boolean(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout bytes.Buffer
	path := "server.enable_tls"
	value := "false"

	err := setConfigValue(&stdout, mockStore, scope, "toml", "", path, value)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get(path); got.(bool) != false {
		t.Errorf("expected %s to be false, got %v (%T)", path, got, got)
	}
}

func TestSetConfigValue_Success_FromFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-set-from-file-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	fileContent := "new value from file"
	if _, err := tmpFile.WriteString(fileContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout bytes.Buffer
	path := "public_dir"
	value := "@" + tmpFile.Name()

	err = setConfigValue(&stdout, mockStore, scope, "toml", "", path, value)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get(path); got != fileContent {
		t.Errorf("expected %s to be %q, got %q", path, fileContent, got)
	}
}

func TestSetConfigValue_Success_CustomDescription(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout bytes.Buffer
	description := "Manual override of server address"

	err := setConfigValue(&stdout, mockStore, scope, "toml", description, "server.addr", `":443"`)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != description {
		t.Errorf("expected save history to contain %q, got %v", description, mockStore.saveHistory)
	}
}

func TestSetConfigValue_Failure_NonexistentPath(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout bytes.Buffer

	err := setConfigValue(&stdout, mockStore, scope, "toml", "", "server.nonexistent", "true")

	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected error to wrap ErrPathNotFound, got %v", err)
	}
}

func TestSetConfigValue_Failure_StoreReadError(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout bytes.Buffer

	err := setConfigValue(&stdout, mockStore, "app", "toml", "", "any.path", "any_value")

	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestSetConfigValue_Failure_MalformedTOML(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte("[server")})
	var stdout bytes.Buffer

	err := setConfigValue(&stdout, mockStore, scope, "toml", "", "any.path", "any_value")

	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}
