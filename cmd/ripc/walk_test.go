package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

const walkTestConf = `
public_dir = "/var/www/public"
[server]
  # Address the server listens on
  addr = ":8080"
  enable_tls = true
[log.batch]
  batch_size = 200
`

func TestWalkConfig_Success_KeepsValue(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader("\n"), mockStore, scope, "server.addr")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save, got %d", len(mockStore.saveHistory))
	}
	for _, want := range []string{"server.addr  (Address the server listens on)", "New value [:8080]: "} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("expected output to contain %q, got %q", want, stdout.String())
		}
	}
	if stderr.String() != "Matched 1 values in scope 'app'\nNo changes.\n" {
		t.Errorf("expected count and no-changes messages, got %q", stderr.String())
	}
}

func TestWalkConfig_Success_UpdatesString(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader("localhost:9999\n"), mockStore, scope, "server.addr")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get("server.addr"); got != "localhost:9999" {
		t.Errorf("server.addr = %v, want localhost:9999", got)
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != "Walk updates" {
		t.Errorf("expected save description %q, got %v", "Walk updates", mockStore.saveHistory)
	}
	if !strings.Contains(stderr.String(), "Updated 1 values in scope 'app'") {
		t.Errorf("expected save message, got %q", stderr.String())
	}
}

func TestWalkConfig_Success_FromFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-walk-from-file-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		removeErr := os.Remove(tmpFile.Name())
		if removeErr != nil {
			t.Logf("warning: failed to remove temp file %s: %v", tmpFile.Name(), removeErr)
		}
	}()

	fileContent := "value from file"
	if _, err := tmpFile.WriteString(fileContent); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err = walkConfig(ui, strings.NewReader("@"+tmpFile.Name()+"\n"), mockStore, scope, "public_dir")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get("public_dir"); got != fileContent {
		t.Errorf("public_dir = %q, want %q", got, fileContent)
	}
}

func TestWalkConfig_Success_UpdatesSeveralValues(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader("localhost:9999\nfalse\n"), mockStore, scope, "server")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	if got := tree.Get("server.addr"); got != "localhost:9999" {
		t.Errorf("server.addr = %v, want localhost:9999", got)
	}
	if got := tree.Get("server.enable_tls"); got != false {
		t.Errorf("server.enable_tls = %v, want false", got)
	}
	if !strings.Contains(stderr.String(), "Matched 2 values in scope 'app'") {
		t.Errorf("expected match count, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Updated 2 values in scope 'app'") {
		t.Errorf("expected save message, got %q", stderr.String())
	}
}

func TestWalkConfig_EOFStopsWalk(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader(""), mockStore, scope, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save, got %d", len(mockStore.saveHistory))
	}
	if stderr.String() != "Matched 4 values in scope 'app'\nNo changes.\n" {
		t.Errorf("expected count and no-changes messages, got %q", stderr.String())
	}
}

func TestWalkConfig_NoMatches(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader(""), mockStore, scope, "absent")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "No TOML paths with values matching 'absent' found in scope 'app'.\n"
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.String() != expected {
		t.Errorf("expected stderr %q, got %q", expected, stderr.String())
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save, got %d", len(mockStore.saveHistory))
	}
}

func TestWalkConfig_Failure_StoreReadError(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader(""), mockStore, "app", "")

	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestWalkConfig_Failure_MalformedTOML(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte("[server")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader(""), mockStore, scope, "")

	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestWalkConfig_Failure_SaveError(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(walkTestConf)})
	mockStore.ForceSaveError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader("localhost:9999\n"), mockStore, scope, "server.addr")

	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

// TestWalkConfig_Failure_ValidationFailure sets a string field to a number.
// The config refuses to unmarshal, so nothing is saved.
func TestWalkConfig_Failure_ValidationFailure(t *testing.T) {
	scope := "app"
	conf := "[acme]\n  profile = \"tlsserver\"\n"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(conf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := walkConfig(ui, strings.NewReader("123\n"), mockStore, scope, "acme.profile")

	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save, got %d", len(mockStore.saveHistory))
	}
}

// TestHandleWalkCommand_Help verifies that -h prints usage to stdout and
// returns nil instead of an error.
func TestHandleWalkCommand_Help(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleWalkCommand(mockStore, strings.NewReader(""), []string{"-h"}, ui)

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
