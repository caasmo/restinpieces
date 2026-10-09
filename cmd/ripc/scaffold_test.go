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
[oauth2]
[scheduler]
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
	if !strings.Contains(stderr.String(), "Successfully scaffolded 'app-online'") {
		t.Errorf("expected success line with backup label, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "app-online:") {
		t.Errorf("expected label block header, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "ripc walk backup.online.app-online") {
		t.Errorf("expected next steps command, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Deactivate: ripc set backup.online.app-online.source_path") {
		t.Errorf("expected deactivate command, got %q", stderr.String())
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

func TestScaffoldConfigValue_BackupS3Upload(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupS3Upload, "app-s3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	path := "backup.s3-upload.app-s3"
	entryTree, ok := tree.Get(path).(*toml.Tree)
	if !ok {
		t.Fatalf("expected subtree at %s", path)
	}
	if !entryTree.Has("path") {
		t.Errorf("expected path field in scaffolded entry")
	}
	if !strings.Contains(stderr.String(), "ripc walk backup.s3-upload.app-s3") {
		t.Errorf("expected next steps command, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Deactivate: ripc set backup.s3-upload.app-s3.path") {
		t.Errorf("expected deactivate command, got %q", stderr.String())
	}
}

func TestScaffoldConfigValue_BackupS3Download(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupS3Download, "app-dl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	path := "backup.s3-download.app-dl"
	entryTree, ok := tree.Get(path).(*toml.Tree)
	if !ok {
		t.Fatalf("expected subtree at %s", path)
	}
	if got := entryTree.Get("min_interval"); got != "5m0s" {
		t.Errorf("expected min_interval 5m, got %v", got)
	}
	if !entryTree.Has("object_key_prefix") {
		t.Errorf("expected object_key_prefix field in scaffolded entry")
	}
	if !strings.Contains(stderr.String(), "ripc walk backup.s3-download.app-dl") {
		t.Errorf("expected next steps command, got %q", stderr.String())
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
	path := "oauth2.my_github"
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
		got := scaffoldNextSteps("backup.sqlite-rsync.entries", "app-rsync", config.NewBackupSqliteRsyncEntryDefaults())
		if !strings.Contains(got, "app-rsync:") {
			t.Fatalf("expected label header, got %q", got)
		}
		if !strings.Contains(got, "\tripc walk backup.sqlite-rsync.entries.app-rsync") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set backup.sqlite-rsync.entries.app-rsync.source_path \"\"") {
			t.Fatalf("expected Deactivate line, got %q", got)
		}
	})
	t.Run("vacuum", func(t *testing.T) {
		got := scaffoldNextSteps("backup.vacuum", "app-vacuum", config.NewBackupVacuumEntryDefaults())
		if !strings.Contains(got, "\tripc walk backup.vacuum.app-vacuum") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set backup.vacuum.app-vacuum.source_path \"\"") {
			t.Fatalf("expected Deactivate line, got %q", got)
		}
	})
	t.Run("online", func(t *testing.T) {
		got := scaffoldNextSteps("backup.online", "app-online", config.NewBackupOnlineAPIEntryDefaults())
		if !strings.Contains(got, "\tripc walk backup.online.app-online") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
	})
	t.Run("s3 upload", func(t *testing.T) {
		got := scaffoldNextSteps("backup.s3-upload", "app-s3", config.NewBackupS3UploadEntryDefaults())
		if !strings.Contains(got, "\tripc walk backup.s3-upload.app-s3") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set backup.s3-upload.app-s3.path \"\"") {
			t.Fatalf("expected path Deactivate line, got %q", got)
		}
		if !strings.Contains(got, "\tripc set backup.s3-upload.app-s3.path_prefix \"\"") {
			t.Fatalf("expected path_prefix Deactivate line, got %q", got)
		}
	})
	t.Run("s3 download", func(t *testing.T) {
		got := scaffoldNextSteps("backup.s3-download", "app-dl", config.NewBackupS3DownloadEntryDefaults())
		if !strings.Contains(got, "\tripc walk backup.s3-download.app-dl") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set backup.s3-download.app-dl.object_key_prefix \"\"") {
			t.Fatalf("expected Deactivate line, got %q", got)
		}
	})
	t.Run("acme dns-01", func(t *testing.T) {
		got := scaffoldNextSteps("acme.dns-01", "my_cf", config.NewAcmeDNS01EntryDefaults())
		if !strings.Contains(got, "\tripc walk acme.dns-01.my_cf") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set acme.dns-01.my_cf.provider \"\"") {
			t.Fatalf("expected Deactivate line, got %q", got)
		}
	})
	t.Run("oauth2", func(t *testing.T) {
		got := scaffoldNextSteps("oauth2", "my_google", config.NewOAuth2EntryDefaults())
		if !strings.Contains(got, "\tripc walk oauth2.my_google") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if strings.Contains(got, "Deactivate") {
			t.Errorf("oauth2 has nothing to turn off, got %q", got)
		}
	})
	t.Run("job", func(t *testing.T) {
		got := scaffoldNextSteps("scheduler.jobs", "acme_cert", config.NewJobEntryDefaults())
		if !strings.Contains(got, "\tripc walk scheduler.jobs.acme_cert") {
			t.Fatalf("expected tab-indented walk command, got %q", got)
		}
		if !strings.Contains(got, "Deactivate: ripc set scheduler.jobs.acme_cert.activated false") {
			t.Fatalf("expected Deactivate line, got %q", got)
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

func TestScaffoldConfigValue_AcmeDNS01(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeAcmeDNS01, "my_cf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	// The scaffold creates the missing [acme] section with the default
	// remaining lifetime fraction, then the entry under dns-01.<label>.
	if got := tree.Get("acme.remaining_lifetime_fraction"); got != 0.25 {
		t.Errorf("expected default remaining lifetime fraction, got %v", got)
	}
	path := "acme.dns-01.my_cf"
	entryTree, ok := tree.Get(path).(*toml.Tree)
	if !ok {
		t.Fatalf("expected subtree at %s", path)
	}
	if got := entryTree.Get("provider"); got != "" {
		t.Errorf("expected empty provider, got %v", got)
	}
	credentials, ok := entryTree.Get("credentials").(*toml.Tree)
	if !ok {
		t.Fatalf("expected credentials subtree at %s.credentials", path)
	}
	if got := credentials.Get("api_token"); got != "" {
		t.Errorf("expected empty api_token, got %v", got)
	}
	if !strings.Contains(stderr.String(), "ripc walk acme.dns-01.my_cf") {
		t.Errorf("expected next steps command, got %q", stderr.String())
	}
}

func TestScaffoldConfigValue_Job(t *testing.T) {
	scope := config.ScopeApplication
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(scaffoldTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeJob, "acme_cert")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tree := getTreeFromStore(t, mockStore, scope)
	path := "scheduler.jobs.acme_cert"
	entryTree, ok := tree.Get(path).(*toml.Tree)
	if !ok {
		t.Fatalf("expected subtree at %s", path)
	}
	if got := entryTree.Get("job_type"); got != "" {
		t.Errorf("expected empty job_type, got %v", got)
	}
	if got := entryTree.Get("interval"); got != "1h0m0s" {
		t.Errorf("expected interval 1h, got %v", got)
	}
	if got := entryTree.Get("activated"); got != false {
		t.Errorf("expected activated false, got %v", got)
	}
	if !strings.Contains(stderr.String(), "ripc walk scheduler.jobs.acme_cert") {
		t.Errorf("expected next steps command, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Deactivate: ripc set scheduler.jobs.acme_cert.activated false") {
		t.Errorf("expected deactivate command, got %q", stderr.String())
	}
}

func TestScaffoldConfigValue_RefusesInvalidConfig(t *testing.T) {
	scope := config.ScopeApplication
	conf := "server = \"oops\"\n[backup]\n"
	mockStore := NewMockSetSecureStore(map[string][]byte{scope: []byte(conf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := scaffoldConfigValue(ui, mockStore, "", ScaffoldTypeBackupVacuum, "app-vacuum")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
	if len(mockStore.saveHistory) != 0 {
		t.Errorf("expected no save on invalid config, got %d saves", len(mockStore.saveHistory))
	}
}
