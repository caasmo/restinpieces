package main

import (
	"bytes"
	"errors"
	"testing"
)

const addTestConf = `
[block_user_agent]
  activated = true
  agents = ["GPTBot"]

[block_host]
  activated = true
  allowed_hosts = ["example.com"]
`

func TestParseAddArgs(t *testing.T) {
	testCases := []struct {
		name         string
		args         []string
		expectedPath string
		expectedVal  string
		expectedErr  error
	}{
		{
			name:        "MissingValue",
			args:        []string{"block_user_agent.agents"},
			expectedErr: ErrMissingArgument,
		},
		{
			name:        "TooManyArgs",
			args:        []string{"block_user_agent.agents", "SemrushBot", "extra"},
			expectedErr: ErrTooManyArguments,
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
			opts, err := parseAddArgs(tc.args)

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

func TestAddValue_NotCollection(t *testing.T) {
	scope := "app"
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_user_agent.activated", "true")
	if !errors.Is(err, ErrNotCollection) {
		t.Fatalf("expected error to wrap ErrNotCollection, got %v", err)
	}
}

func TestAddValue_Failure_MissingPath(t *testing.T) {
	scope := "app"
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "server.addr", ":9090")
	if !errors.Is(err, ErrPathNotFound) {
		t.Fatalf("expected error to wrap ErrPathNotFound, got %v", err)
	}
}

func TestAddValue_Failure_MalformedTOML(t *testing.T) {
	scope := "app"
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte("[block_user_agent")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_user_agent.agents", "SemrushBot")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestAddValue_Failure_StoreGetError(t *testing.T) {
	mockStore := NewMockGenSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, "app", "", "block_user_agent.agents", "SemrushBot")
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Fatalf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestAddValue_Failure_StoreSaveError(t *testing.T) {
	scope := "app"
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	mockStore.ForceSaveError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_user_agent.agents", "SemrushBot")
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Fatalf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

func TestHandleAddCommand_Help(t *testing.T) {
	mockStore := NewMockGenSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleAddCommand(mockStore, []string{"-h"}, ui)
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
