package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

const addTestConf = `
[block_ua_list]
  activated = true
  list = "(GPTBot)"
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
			args:        []string{"block_ua_list.list"},
			expectedErr: ErrMissingArgument,
		},
		{
			name:        "TooManyArgs",
			args:        []string{"block_ua_list.list", "SemrushBot", "extra"},
			expectedErr: ErrTooManyArguments,
		},
		{
			name:         "PathAndValue",
			args:         []string{"block_ua_list.list", "SemrushBot"},
			expectedPath: "block_ua_list.list",
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

func TestAddValue_UserAgent(t *testing.T) {
	scope := "app"
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_ua_list.list", "SemrushBot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getGenTreeFromStore(t, mockStore, scope)
	got, ok := tree.Get("block_ua_list.list").(string)
	if !ok {
		t.Fatalf("expected %s to be a string, got %T", "block_ua_list.list", tree.Get("block_ua_list.list"))
	}
	_, compileErr := regexp.Compile(got)
	if compileErr != nil {
		t.Fatalf("expected result to compile, got error: %v", compileErr)
	}
	for _, want := range []string{"GPTBot", "SemrushBot"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestAddValue_NotCollection(t *testing.T) {
	scope := "app"
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_ua_list.activated", "true")
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
	mockStore := NewMockGenSecureStore(map[string][]byte{scope: []byte("[block_ua_list")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_ua_list.list", "SemrushBot")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestAddValue_Failure_StoreGetError(t *testing.T) {
	mockStore := NewMockGenSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, "app", "", "block_ua_list.list", "SemrushBot")
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

	err := addValue(ui, mockStore, scope, "", "block_ua_list.list", "SemrushBot")
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
