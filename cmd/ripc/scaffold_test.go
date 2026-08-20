package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

const scaffoldTestConf = `
public_dir = "/var/www/public"
[server]
  addr = ":8080"
[backup]
[oauth2_providers]
`

func TestScaffoldConfigValue_Backup(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, scope, "", ScaffoldTypeBackup, "app_db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getTreeFromStore(t, mockStore, scope)
	path := "backup.files.app_db"
	if !tree.Has(path) {
		t.Fatalf("expected path %s to exist", path)
	}
	filesTree, ok := tree.Get(path).(*toml.Tree)
	if !ok {
		t.Fatalf("expected subtree at %s", path)
	}
	if got := filesTree.Get("strategy"); got != config.BackupStrategyOnline {
		t.Errorf("expected strategy %q, got %v", config.BackupStrategyOnline, got)
	}
	if got := filesTree.Get("compression"); got != false {
		t.Errorf("expected compression false, got %v", got)
	}
	if got := filesTree.Get("frequency"); got != "15m0s" {
		t.Errorf("expected frequency %q, got %v", "15m0s", got)
	}
	if got := filesTree.Get("online_api_pages_per_step"); got != int64(100) {
		t.Errorf("expected online_api_pages_per_step 100, got %v", got)
	}
	if got := filesTree.Get("online_api_sleep_interval"); got != "10ms" {
		t.Errorf("expected online_api_sleep_interval %q, got %v", "10ms", got)
	}
}

func TestScaffoldConfigValue_OAuth2(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, scope, "", ScaffoldTypeOAuth2, "my_github")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getTreeFromStore(t, mockStore, scope)
	path := "oauth2_providers.my_github"
	if !tree.Has(path) {
		t.Fatalf("expected path %s to exist", path)
	}
	subtree, _ := tree.Get(path).(*toml.Tree)
	if got := subtree.Get("pkce"); got != true {
		t.Errorf("expected pkce true, got %v", got)
	}
}

func TestScaffoldConfigValue_UnknownType(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "app", "", "bogus", "key")
	if !errors.Is(err, ErrScaffoldTypeUnknown) {
		t.Errorf("expected ErrScaffoldTypeUnknown, got %v", err)
	}
}

func TestScaffoldConfigValue_KeyExists(t *testing.T) {
	tomlWithBackup := scaffoldTestConf + "\n[backup.files.app_db]\n  source_path = \"/x.db\"\n"
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(tomlWithBackup)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, scope, "", ScaffoldTypeBackup, "app_db")
	if !errors.Is(err, ErrScaffoldKeyExists) {
		t.Errorf("expected ErrScaffoldKeyExists, got %v", err)
	}
}

func TestScaffoldConfigValue_StoreReadError(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "app", "", ScaffoldTypeBackup, "app_db")
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestScaffoldConfigValue_MalformedTOML(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte("[server")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, scope, "", ScaffoldTypeBackup, "app_db")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestScaffoldConfigValue_StoreSaveError(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	mockStore.ForceSaveError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, scope, "", ScaffoldTypeBackup, "app_db")
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

func TestScaffoldConfigValue_CustomDescription(t *testing.T) {
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	desc := "scaffolding analytics db"

	err := scaffoldConfigValue(ui, mockStore, scope, desc, ScaffoldTypeBackup, "analytics_db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != desc {
		t.Errorf("expected save description %q, got %v", desc, mockStore.saveHistory)
	}
}

func TestScaffoldConfigValue_ParentMissing(t *testing.T) {
	// setTestConf has no [backup] section
	scope := "app"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, scope, "", ScaffoldTypeBackup, "app_db")
	if !errors.Is(err, ErrScaffoldParentMissing) {
		t.Errorf("expected ErrScaffoldParentMissing, got %v", err)
	}
}

func TestParseScaffoldArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantScope      string
		wantType       string
		wantKey        string
		wantErrContain string
	}{
		{
			name:      "two positional args",
			args:      []string{"backup", "app_db"},
			wantScope: config.ScopeApplication,
			wantType:  "backup",
			wantKey:   "app_db",
		},
		{
			name:      "flags before positional",
			args:      []string{"--scope", "my-app", "backup", "app_db"},
			wantScope: "my-app",
			wantType:  "backup",
			wantKey:   "app_db",
		},
		{
			name:           "missing key arg",
			args:           []string{"backup"},
			wantErrContain: "requires <type> and <key>",
		},
		{
			name:           "flags after positional (not consumed)",
			args:           []string{"backup", "app_db", "--scope", "my-app"},
			wantErrContain: "takes exactly two arguments",
		},
		{
			name:           "too many positional",
			args:           []string{"backup", "app_db", "extra"},
			wantErrContain: "takes exactly two arguments",
		},
		{
			name:           "unknown flag",
			args:           []string{"--bogus", "backup", "app_db"},
			wantErrContain: "flag provided but not defined",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseScaffoldArgs(tc.args)
			if tc.wantErrContain != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrContain)
				}
				if !strings.Contains(err.Error(), tc.wantErrContain) {
					t.Errorf("error %q should contain %q", err.Error(), tc.wantErrContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Scope != tc.wantScope {
				t.Errorf("scope: got %q, want %q", opts.Scope, tc.wantScope)
			}
			if opts.ScaffoldType != tc.wantType {
				t.Errorf("type: got %q, want %q", opts.ScaffoldType, tc.wantType)
			}
			if opts.Key != tc.wantKey {
				t.Errorf("key: got %q, want %q", opts.Key, tc.wantKey)
			}
		})
	}
}

// TestHandleScaffoldCommand_Help verifies that -h prints usage to stdout and
// returns nil instead of an error.
func TestHandleScaffoldCommand_Help(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleScaffoldCommand(mockStore, []string{"-h"}, ui)

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
