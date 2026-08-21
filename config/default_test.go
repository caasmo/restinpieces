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

func TestNewBackupOnlineDefaults(t *testing.T) {
	v := NewBackupOnlineDefaults()
	if v.SourcePath != "" {
		t.Errorf("SourcePath: got %q, want empty", v.SourcePath)
	}
	if v.DestPath != "" {
		t.Errorf("DestPath: got %q, want empty", v.DestPath)
	}
	if v.Strategy != BackupStrategyOnline {
		t.Errorf("Strategy: got %q, want %q", v.Strategy, BackupStrategyOnline)
	}
	if v.Compression != false {
		t.Errorf("Compression: got %v, want false", v.Compression)
	}
	if v.Frequency.Duration != 15*time.Minute {
		t.Errorf("Frequency: got %v, want 15m", v.Frequency)
	}
	if v.OnlineAPIPagesPerStep != 100 {
		t.Errorf("OnlineAPIPagesPerStep: got %d, want 100", v.OnlineAPIPagesPerStep)
	}
	if v.OnlineAPISleepInterval.Duration != 10*time.Millisecond {
		t.Errorf("OnlineAPISleepInterval: got %v, want 10ms", v.OnlineAPISleepInterval)
	}
}

func TestNewBackupVacuumDefaults(t *testing.T) {
	v := NewBackupVacuumDefaults()
	if v.Strategy != BackupStrategyVacuum {
		t.Errorf("Strategy: got %q, want %q", v.Strategy, BackupStrategyVacuum)
	}
	if v.Compression != false {
		t.Errorf("Compression: got %v, want false", v.Compression)
	}
	if v.Frequency.Duration != 15*time.Minute {
		t.Errorf("Frequency: got %v, want 15m", v.Frequency)
	}
	if v.OnlineAPIPagesPerStep != 0 {
		t.Errorf("OnlineAPIPagesPerStep: got %d, want 0", v.OnlineAPIPagesPerStep)
	}
}

func TestNewBackupSqliteRsyncDefaults(t *testing.T) {
	v := NewBackupSqliteRsyncDefaults()
	if v.Strategy != BackupStrategySqliteRsync {
		t.Errorf("Strategy: got %q, want %q", v.Strategy, BackupStrategySqliteRsync)
	}
	if v.SyncTimeout.Duration != 15*time.Minute {
		t.Errorf("SyncTimeout: got %v, want 15m", v.SyncTimeout)
	}
	if v.Frequency.Duration != 0 {
		t.Errorf("Frequency: got %v, want 0", v.Frequency)
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
