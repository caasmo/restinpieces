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

func TestNewBackupLocalDbFileDefaults(t *testing.T) {
	v := NewBackupLocalDbFileDefaults()
	if v.SourcePath != "" {
		t.Errorf("SourcePath: got %q, want empty", v.SourcePath)
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
