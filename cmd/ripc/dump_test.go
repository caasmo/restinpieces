package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml/v2"
)

// This MockSecureStore is a simplified version for dump command tests.
// A shared test utility could be a future improvement.
type MockDumpSecureStore struct {
	data          map[string][]byte
	ForceGetError bool
}

func NewMockDumpSecureStore(initialData map[string][]byte) *MockDumpSecureStore {
	if initialData == nil {
		initialData = make(map[string][]byte)
	}
	return &MockDumpSecureStore{
		data: initialData,
	}
}

func (m *MockDumpSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.ForceGetError {
		return nil, "", fmt.Errorf("forced get error: %w", ErrSecureStoreGet)
	}
	data, ok := m.data[scope]
	if !ok {
		return []byte{}, "toml", nil
	}
	return data, "toml", nil
}

// Save is a no-op for dump tests but required to satisfy the SecureStore interface.
func (m *MockDumpSecureStore) Save(scope string, data []byte, format string, description string) error {
	return nil
}

// TestDumpConfig_Success verifies successful writing of config data.
func TestDumpConfig_Success(t *testing.T) {
	scope := "test_app"
	storedData := `[server]
addr = ":9090"
`
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		scope: []byte(storedData),
	})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := dumpConfig(ui, mockStore, scope, false, false)

	if err != nil {
		t.Fatalf("dumpConfig() returned an unexpected error: %v", err)
	}
	got := stdout.String()
	if got != storedData {
		t.Errorf("dumpConfig() output = %q, want %q", got, storedData)
	}
}

// TestDumpConfig_DefaultScope verifies use of the default application scope.
func TestDumpConfig_DefaultScope(t *testing.T) {
	storedData := `[server]
addr = ":9090"
`
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		config.ScopeApplication: []byte(storedData),
	})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := dumpConfig(ui, mockStore, "", false, false)

	if err != nil {
		t.Fatalf("dumpConfig() with empty scope returned an unexpected error: %v", err)
	}
	got := stdout.String()
	if got != storedData {
		t.Errorf("dumpConfig() output = %q, want %q", got, storedData)
	}
}

// TestDumpConfig_Failure_StoreReadError tests failure on store read error.
func TestDumpConfig_Failure_StoreReadError(t *testing.T) {
	mockStore := NewMockDumpSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := dumpConfig(ui, mockStore, "any_scope", false, false)

	if err == nil {
		t.Fatal("dumpConfig() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("dumpConfig() error = %v, want error wrapping %v", err, ErrSecureStoreGet)
	}
}

// TestDumpConfig_Failure_OutputWriteError tests write-failure for all dump modes.
func TestDumpConfig_Failure_OutputWriteError(t *testing.T) {
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		"any_scope": []byte("[server]\naddr = ':8080'"),
	})

	tests := []struct {
		name    string
		zero    bool
		runtime bool
	}{
		{"raw", false, false},
		{"zero", true, false},
		{"runtime", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var failingStdout failingWriter
			ui := UI{Out: &failingStdout, Err: io.Discard}
			err := dumpConfig(ui, mockStore, "any_scope", tc.zero, tc.runtime)
			if err == nil {
				t.Fatal("dumpConfig() was expected to return an error, but did not")
			}
			if !errors.Is(err, ErrWriteOutput) {
				t.Errorf("dumpConfig(%s) error = %v, want error wrapping %v", tc.name, err, ErrWriteOutput)
			}
		})
	}
}

// failingWriter is an io.Writer that always returns an error.
type failingWriter struct{}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("forced write error")
}

// TestDumpConfig_Runtime verifies that runtime dump merges defaults with stored overrides.
func TestDumpConfig_Runtime(t *testing.T) {
	scope := "test_app"
	override := `[server]
addr = ":9090"
`
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		scope: []byte(override),
	})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := dumpConfig(ui, mockStore, scope, false, true)
	if err != nil {
		t.Fatalf("dumpConfig(runtime) returned an unexpected error: %v", err)
	}

	var got config.Config
	err = toml.Unmarshal(stdout.Bytes(), &got)
	if err != nil {
		t.Fatalf("dumpConfig(runtime) produced invalid TOML: %v", err)
	}
	if got.Server.Addr != ":9090" {
		t.Errorf("expected server.addr = %q, got %q", ":9090", got.Server.Addr)
	}
	if got.PublicDir != "static/dist" {
		t.Errorf("expected default public_dir = %q, got %q", "static/dist", got.PublicDir)
	}
}

// TestDumpConfig_RawEmpty verifies raw mode on empty stored data produces no output.
func TestDumpConfig_RawEmpty(t *testing.T) {
	mockStore := NewMockDumpSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := dumpConfig(ui, mockStore, "nonexistent", false, false)

	if err != nil {
		t.Fatalf("dumpConfig(raw empty) returned an unexpected error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("dumpConfig(raw empty) expected empty output, got %q", stdout.String())
	}
}

// TestDumpConfig_Zero verifies zero dump writes stored overrides on a zero-valued config.
func TestDumpConfig_Zero(t *testing.T) {
	scope := "test_app"
	override := `[server]
addr = ":9090"
`
	mockStore := NewMockDumpSecureStore(map[string][]byte{
		scope: []byte(override),
	})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := dumpConfig(ui, mockStore, scope, true, false)
	if err != nil {
		t.Fatalf("dumpConfig(zero) returned an unexpected error: %v", err)
	}

	var got config.Config
	err = toml.Unmarshal(stdout.Bytes(), &got)
	if err != nil {
		t.Fatalf("dumpConfig(zero) produced invalid TOML: %v", err)
	}
	if got.Server.Addr != ":9090" {
		t.Errorf("expected server.addr = %q, got %q", ":9090", got.Server.Addr)
	}
	if got.PublicDir != "" {
		t.Errorf("expected zero-valued public_dir, got %q", got.PublicDir)
	}
}
