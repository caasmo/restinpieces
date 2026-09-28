package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	toml "github.com/pelletier/go-toml"
)

// MockRemoveSecureStore is a test-only implementation of config.SecureStore for remove command tests.
type MockRemoveSecureStore struct {
	data           map[string][]byte
	format         string
	saveHistory    []string
	ForceGetError  bool
	ForceSaveError bool
}

func NewMockRemoveSecureStore(initialData map[string][]byte) *MockRemoveSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockRemoveSecureStore{
		data:   initialData,
		format: "toml",
	}
}

func (m *MockRemoveSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, m.format, nil
	}
	return data, m.format, nil
}

func (m *MockRemoveSecureStore) Save(scope string, data []byte, format string, description string) error {
	if m.ForceSaveError {
		return fmt.Errorf("forced save error: %w", ErrSecureStoreSave)
	}
	m.data[scope] = data
	m.format = format
	m.saveHistory = append(m.saveHistory, description)
	return nil
}

func getRemoveTreeFromStore(t *testing.T, store *MockRemoveSecureStore, scope string) *toml.Tree {
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

const removeTestConf = `
[backup.vacuum.logs-vacuum]
  source_path = ""
  dest_path = ""
  frequency = "15m"
  compression = false

[block_user_agent]
  activated = true
  agents = ["GPTBot", "SemrushBot"]

[block_host]
  activated = true
  allowed_hosts = ["example.com"]
`

func TestParseRemoveArgs(t *testing.T) {
	testCases := []struct {
		name         string
		args         []string
		expectedPath string
		expectedVal  string
		expectedErr  error
	}{
		{
			name:        "MissingPath",
			args:        []string{},
			expectedErr: ErrMissingArgument,
		},
		{
			name:        "TooManyArgs",
			args:        []string{"block_user_agent.agents", "SemrushBot", "extra"},
			expectedErr: ErrTooManyArguments,
		},
		{
			name:         "PathOnly",
			args:         []string{"backup.vacuum.logs-vacuum"},
			expectedPath: "backup.vacuum.logs-vacuum",
			expectedErr:  nil,
		},
		{
			name:         "PathAndValue",
			args:         []string{"block_user_agent.agents", "SemrushBot"},
			expectedPath: "block_user_agent.agents",
			expectedVal:  "SemrushBot",
			expectedErr:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseRemoveArgs(tc.args)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatal("expected error, but got nil")
				}
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error to wrap %v, but got %v", tc.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Path != tc.expectedPath {
				t.Errorf("expected path %q, but got %q", tc.expectedPath, opts.Path)
			}
			if opts.Value != tc.expectedVal {
				t.Errorf("expected value %q, but got %q", tc.expectedVal, opts.Value)
			}
		})
	}
}

func TestRemoveValue_TableEntry(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "backup.vacuum.logs-vacuum", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getRemoveTreeFromStore(t, mockStore, scope)
	if tree.Has("backup.vacuum.logs-vacuum") {
		t.Error("expected backup.vacuum.logs-vacuum to be removed")
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != "Removed 'backup.vacuum.logs-vacuum'" {
		t.Errorf("expected default save description, got %v", mockStore.saveHistory)
	}
}

func TestRemoveValue_ArrayItem(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "block_user_agent.agents", "SemrushBot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getRemoveTreeFromStore(t, mockStore, scope)
	raw, ok := tree.Get("block_user_agent.agents").([]interface{})
	if !ok {
		t.Fatalf("expected block_user_agent.agents to be an array, got %T", tree.Get("block_user_agent.agents"))
	}
	if len(raw) != 1 || raw[0] != "GPTBot" {
		t.Errorf("expected [GPTBot], got %v", raw)
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != "Removed 'SemrushBot' from 'block_user_agent.agents'" {
		t.Errorf("expected default save description, got %v", mockStore.saveHistory)
	}
}

func TestRemoveValue_Failure_MissingValueForArray(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "block_user_agent.agents", "")
	if !errors.Is(err, ErrMissingArgument) {
		t.Fatalf("expected error to wrap ErrMissingArgument, got %v", err)
	}
}

func TestRemoveValue_Failure_ValueNotInArray(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "block_user_agent.agents", "UnknownBot")
	if !errors.Is(err, ErrValueNotFound) {
		t.Fatalf("expected error to wrap ErrValueNotFound, got %v", err)
	}
}

func TestRemoveValue_Failure_ValueOnTableEntry(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "backup.vacuum.logs-vacuum", "extra")
	if !errors.Is(err, ErrTooManyArguments) {
		t.Fatalf("expected error to wrap ErrTooManyArguments, got %v", err)
	}
}

func TestRemoveValue_Failure_ScalarPath(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "block_user_agent.activated", "")
	if !errors.Is(err, ErrNotCollection) {
		t.Fatalf("expected error to wrap ErrNotCollection, got %v", err)
	}
}

func TestRemoveValue_Failure_MissingPath(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "backup.vacuum.unknown", "")
	if !errors.Is(err, ErrPathNotFound) {
		t.Fatalf("expected error to wrap ErrPathNotFound, got %v", err)
	}
}

func TestRemoveValue_Failure_MalformedTOML(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte("[block_user_agent")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "block_user_agent.agents", "SemrushBot")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestRemoveValue_Failure_StoreGetError(t *testing.T) {
	mockStore := NewMockRemoveSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, "app", "", "block_user_agent.agents", "SemrushBot")
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Fatalf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestRemoveValue_Failure_StoreSaveError(t *testing.T) {
	scope := "app"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(removeTestConf)})
	mockStore.ForceSaveError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "block_user_agent.agents", "SemrushBot")
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Fatalf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

func TestRemoveValue_RefusesInvalidConfig(t *testing.T) {
	scope := "app"
	conf := "server = \"oops\"\n[backup.vacuum.logs-vacuum]\n  source_path = \"\"\n"
	mockStore := NewMockRemoveSecureStore(map[string][]byte{scope: []byte(conf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := removeValue(ui, mockStore, scope, "", "backup.vacuum.logs-vacuum", "")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save on invalid config, got %d saves", len(mockStore.saveHistory))
	}
}

func TestHandleRemoveCommand_Help(t *testing.T) {
	mockStore := NewMockRemoveSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleRemoveCommand(mockStore, []string{"-h"}, ui)
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
