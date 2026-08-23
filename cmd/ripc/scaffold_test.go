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

func TestScaffoldConfigValue_BackupOnline(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupOnline, "app-online")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	path := "backup.online.app-online"
	filesTree, ok := tree.Get(path).(*toml.Tree)
	if !ok {
		t.Fatalf("expected subtree at %s", path)
	}
	if got := filesTree.Get("frequency"); got != "15m0s" {
		t.Errorf("expected frequency %q, got %v", "15m0s", got)
	}
	if filesTree.Has("strategy") {
		t.Errorf("online entry should not scaffold strategy field")
	}
	if got := filesTree.Get("pages_per_step"); got != int64(100) {
		t.Errorf("expected 100, got %v", got)
	}
	if !strings.Contains(stderr.String(), "Successfully scaffolded backup 'app-online'") {
		t.Errorf("expected success line with backup label, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "app-online:") {
		t.Errorf("expected label block header, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "ripc set backup.online.app-online.source_path") {
		t.Errorf("expected next steps command, got %q", stderr.String())
	}
}

func TestScaffoldConfigValue_BackupVacuum(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupVacuum, "app-vacuum")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	path := "backup.vacuum.app-vacuum"
	filesTree, _ := tree.Get(path).(*toml.Tree)
	if got := filesTree.Get("frequency"); got != "15m0s" {
		t.Errorf("expected frequency 15m, got %v", got)
	}
	if filesTree.Has("strategy") {
		t.Errorf("vacuum should not scaffold strategy field")
	}
	if filesTree.Has("pages_per_step") {
		t.Errorf("vacuum should not scaffold online tuning")
	}
}

func TestScaffoldConfigValue_BackupSqliteRsync(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupSqliteRsync, "app-rsync")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	// The scaffold creates the missing [backup.sqlite-rsync] section with
	// the default listen_addr, then the entry under entries.<label>.
	if got := tree.Get("backup.sqlite-rsync.listen_addr"); got != "127.0.0.1:54321" {
		t.Errorf("expected default listen_addr, got %v", got)
	}
	path := "backup.sqlite-rsync.entries.app-rsync"
	filesTree, _ := tree.Get(path).(*toml.Tree)
	if got := filesTree.Get("sync_timeout"); got != "15m0s" {
		t.Errorf("expected sync_timeout 15m, got %v", got)
	}
	if filesTree.Has("strategy") {
		t.Errorf("sqlite-rsync should not scaffold strategy field")
	}
	if filesTree.Has("frequency") {
		t.Errorf("rsync should not scaffold frequency")
	}
}

func TestScaffoldConfigValue_LabelWithSpaceRejected(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupSqliteRsync, "my label")
	if err == nil {
		t.Fatal("expected error for label with space")
	}
}

func TestParseScaffoldArgs_RejectsLabelWithWhitespace(t *testing.T) {
	_, err := parseScaffoldArgs([]string{ScaffoldTypeBackupSqliteRsync, "my label"})
	if err == nil {
		t.Fatal("expected error for label with space via parse")
	}
	_, err = parseScaffoldArgs([]string{ScaffoldTypeBackupOnline, "my.label"})
	if err == nil {
		t.Fatal("expected error for label with dot via parse")
	}
	_, err = parseScaffoldArgs([]string{"--scope", "my-app", ScaffoldTypeBackupOnline, "app-online"})
	if err == nil {
		t.Fatal("expected error for --scope flag — scaffold does not support scope")
	}
}

func TestScaffoldConfigValue_OAuth2(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeOAuth2, "my_github")
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

	err := scaffoldConfigValue(ui, mockStore, "", "bogus", "key")
	if !errors.Is(err, ErrScaffoldTypeUnknown) {
		t.Errorf("expected ErrScaffoldTypeUnknown, got %v", err)
	}
}

func TestScaffoldConfigValue_KeyExists(t *testing.T) {
	tomlWithBackup := scaffoldTestConf + "\n[backup.online.app_db]\n  source_path = \"/x.db\"\n"
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(tomlWithBackup)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupOnline, "app_db")
	if !errors.Is(err, ErrScaffoldKeyExists) {
		t.Errorf("expected ErrScaffoldKeyExists, got %v", err)
	}
}

func TestScaffoldConfigValue_StoreReadError(t *testing.T) {
	mockStore := NewMockSetSecureStore(nil)
	mockStore.ForceGetError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupOnline, "app_db")
	if !errors.Is(err, ErrSecureStoreGet) {
		t.Errorf("expected error to wrap ErrSecureStoreGet, got %v", err)
	}
}

func TestScaffoldConfigValue_MalformedTOML(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte("[server")})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupOnline, "app_db")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Errorf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}

func TestScaffoldConfigValue_StoreSaveError(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	mockStore.ForceSaveError = true
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupOnline, "app_db")
	if !errors.Is(err, ErrSecureStoreSave) {
		t.Errorf("expected error to wrap ErrSecureStoreSave, got %v", err)
	}
}

func TestScaffoldConfigValue_CustomDescription(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	desc := "scaffolding analytics db"

	err := scaffoldConfigValue(ui, mockStore, desc, ScaffoldTypeBackupVacuum, "analytics_db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockStore.saveHistory) == 0 || mockStore.saveHistory[0] != desc {
		t.Errorf("expected save description %q, got %v", desc, mockStore.saveHistory)
	}
}

func TestScaffoldConfigValue_ParentMissing(t *testing.T) {
	// setTestConf has no [backup] section
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(setTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupVacuum, "app_db")
	if !errors.Is(err, ErrScaffoldParentMissing) {
		t.Errorf("expected ErrScaffoldParentMissing, got %v", err)
	}
}

func TestParseScaffoldArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantType       string
		wantKey        string
		wantErrContain string
	}{
		{
			name:     "two positional args",
			args:     []string{ScaffoldTypeBackupOnline, "app-online"},
			wantType: ScaffoldTypeBackupOnline,
			wantKey:  "app-online",
		},
		{
			name:           "missing key arg",
			args:           []string{ScaffoldTypeBackupOnline},
			wantErrContain: "requires <type> and <key>",
		},
		{
			name:           "flags after positional (not consumed)",
			args:           []string{ScaffoldTypeBackupOnline, "app-online", "--desc", "hi"},
			wantErrContain: "takes exactly two arguments",
		},
		{
			name:           "too many positional",
			args:           []string{ScaffoldTypeBackupOnline, "app-online", "extra"},
			wantErrContain: "takes exactly two arguments",
		},
		{
			name:           "unknown flag",
			args:           []string{"--bogus", ScaffoldTypeBackupOnline, "app-online"},
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
			if opts.ScaffoldType != tc.wantType {
				t.Errorf("type: got %q, want %q", opts.ScaffoldType, tc.wantType)
			}
			if opts.Key != tc.wantKey {
				t.Errorf("key: got %q, want %q", opts.Key, tc.wantKey)
			}
		})
	}
}

func TestScaffoldNextSteps(t *testing.T) {
	t.Run("rsync", func(t *testing.T) {
		got := scaffoldNextSteps(ScaffoldTypeBackupSqliteRsync, "app-rsync", config.NewBackupSqliteRsyncEntryDefaults())
		if !strings.Contains(got, "app-rsync:") {
			t.Fatalf("expected label header, got %q", got)
		}
		if !strings.Contains(got, "\tripc set backup.sqlite-rsync.entries.app-rsync.source_path") {
			t.Fatalf("expected tab-indented command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set backup.sqlite-rsync.entries.app-rsync.source_path") {
			t.Fatalf("expected Deactivate line, got %q", got)
		}
	})
	t.Run("vacuum", func(t *testing.T) {
		got := scaffoldNextSteps(ScaffoldTypeBackupVacuum, "app-vacuum", config.NewBackupVacuumEntryDefaults())
		if !strings.Contains(got, "\tripc set backup.vacuum.app-vacuum.dest_path") {
			t.Fatalf("expected dest_path command, got %q", got)
		}
	})
	t.Run("online", func(t *testing.T) {
		got := scaffoldNextSteps(ScaffoldTypeBackupOnline, "app-online", config.NewBackupOnlineAPIEntryDefaults())
		if !strings.Contains(got, "\tripc set backup.online.app-online.frequency 24h") {
			t.Fatalf("expected frequency command, got %q", got)
		}
	})
	t.Run("oauth2 empty", func(t *testing.T) {
		got := scaffoldNextSteps(ScaffoldTypeOAuth2, "my_google", config.NewOAuth2ProviderDefaults())
		if got != "" {
			t.Fatalf("expected empty for oauth2, got %q", got)
		}
	})
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
