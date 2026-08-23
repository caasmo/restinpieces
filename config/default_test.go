package config

import (
	"testing"
	"time"
)

func TestNewDefaultConfig_DeterministicSecrets(t *testing.T) {
	cfg := NewDefaultConfig()

	checks := map[string]string{
		"AuthSecret":                 cfg.Jwt.AuthSecret,
		"PasswordResetSecret":        cfg.Jwt.PasswordResetSecret,
		"EmailChangeOtpSecret":       cfg.Jwt.EmailChangeOtpSecret,
		"VerificationEmailOtpSecret": cfg.Jwt.VerificationEmailOtpSecret,
		"Oauth2StateSecret":          cfg.Jwt.Oauth2StateSecret,
	}
	for name, val := range checks {
		if val != "" {
			t.Errorf("%s: got %q, want empty (deterministic)", name, val)
		}
	}
}

func TestNewBackupOnlineAPIEntryDefaults(t *testing.T) {
	v := NewBackupOnlineAPIEntryDefaults()
	if v.SourcePath != "" {
		t.Errorf("SourcePath: got %q, want empty", v.SourcePath)
	}
	if v.DestPath != "" {
		t.Errorf("DestPath: got %q, want empty", v.DestPath)
	}
	if v.Compression != false {
		t.Errorf("Compression: got %v, want false", v.Compression)
	}
	if v.Frequency.Duration != 15*time.Minute {
		t.Errorf("Frequency: got %v, want 15m", v.Frequency)
	}
	if v.PagesPerStep != 100 {
		t.Errorf("PagesPerStep: got %d, want 100", v.PagesPerStep)
	}
	if v.SleepInterval.Duration != 10*time.Millisecond {
		t.Errorf("SleepInterval: got %v, want 10ms", v.SleepInterval)
	}
}

func TestNewBackupVacuumEntryDefaults(t *testing.T) {
	v := NewBackupVacuumEntryDefaults()
	if v.SourcePath != "" {
		t.Errorf("SourcePath: got %q, want empty", v.SourcePath)
	}
	if v.DestPath != "" {
		t.Errorf("DestPath: got %q, want empty", v.DestPath)
	}
	if v.Compression != false {
		t.Errorf("Compression: got %v, want false", v.Compression)
	}
	if v.Frequency.Duration != 15*time.Minute {
		t.Errorf("Frequency: got %v, want 15m", v.Frequency)
	}
}

func TestNewBackupSqliteRsyncEntryDefaults(t *testing.T) {
	v := NewBackupSqliteRsyncEntryDefaults()
	if v.SourcePath != "" {
		t.Errorf("SourcePath: got %q, want empty", v.SourcePath)
	}
	if v.SyncTimeout.Duration != 15*time.Minute {
		t.Errorf("SyncTimeout: got %v, want 15m", v.SyncTimeout)
	}
}

func TestNewBackupSqliteRsyncDefaults(t *testing.T) {
	v := NewBackupSqliteRsyncDefaults()
	if v.ListenAddr != "127.0.0.1:54321" {
		t.Errorf("ListenAddr: got %q, want %q", v.ListenAddr, "127.0.0.1:54321")
	}
	if v.Entries != nil {
		t.Errorf("Entries: got %v, want nil", v.Entries)
	}
}

func TestNewDefaultConfigWiresSqliteRsyncDefaults(t *testing.T) {
	cfg := NewDefaultConfig()
	if got := cfg.Backup.SqliteRsync.ListenAddr; got != "127.0.0.1:54321" {
		t.Errorf("Backup.SqliteRsync.ListenAddr: got %q, want %q", got, "127.0.0.1:54321")
	}
}

func TestNewOAuth2ProviderDefaults(t *testing.T) {
	v := NewOAuth2ProviderDefaults()
	if v.Name != "" {
		t.Errorf("Name: got %q, want empty", v.Name)
	}
	if v.ClientID != "" {
		t.Errorf("ClientID: got %q, want empty", v.ClientID)
	}
	if v.ClientSecret != "" {
		t.Errorf("ClientSecret: got %q, want empty", v.ClientSecret)
	}
	if v.PKCE != true {
		t.Errorf("PKCE: got %v, want true", v.PKCE)
	}
}
