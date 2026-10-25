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

func TestNewBackupS3UploadEntryDefaults(t *testing.T) {
	v := NewBackupS3UploadEntryDefaults()
	if v.Path != "" {
		t.Errorf("Path: got %q, want empty", v.Path)
	}
	if v.PathPrefix != "" {
		t.Errorf("PathPrefix: got %q, want empty", v.PathPrefix)
	}
	if v.PathPrefixSelector != "latest" {
		t.Errorf("PathPrefixSelector: got %q, want latest", v.PathPrefixSelector)
	}
	if v.AgeRecipient != "" {
		t.Errorf("AgeRecipient: got %q, want empty", v.AgeRecipient)
	}
}

func TestNewBackupS3DownloadEntryDefaults(t *testing.T) {
	v := NewBackupS3DownloadEntryDefaults()
	if v.ObjectKeyPrefix != "" {
		t.Errorf("ObjectKeyPrefix: got %q, want empty", v.ObjectKeyPrefix)
	}
	if v.Bucket != "" {
		t.Errorf("Bucket: got %q, want empty", v.Bucket)
	}
	if v.DestDir != "" {
		t.Errorf("DestDir: got %q, want empty", v.DestDir)
	}
	if v.MinInterval.Duration != 5*time.Minute {
		t.Errorf("MinInterval: got %v, want 5m", v.MinInterval)
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

func TestNewOAuth2EntryDefaults(t *testing.T) {
	v := NewOAuth2EntryDefaults()
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

func TestNewAcmeDefaults(t *testing.T) {
	v := NewAcmeDefaults()
	if v.RemainingLifetimeFraction != 0.25 {
		t.Errorf("RemainingLifetimeFraction: got %v, want %v", v.RemainingLifetimeFraction, 0.25)
	}
	if v.DNS01 != nil {
		t.Errorf("DNS01: got %v, want nil", v.DNS01)
	}
}

func TestNewAcmeDNS01EntryDefaults(t *testing.T) {
	v := NewAcmeDNS01EntryDefaults()
	if v.Provider != "" {
		t.Errorf("Provider: got %q, want empty", v.Provider)
	}
	if _, ok := v.Credentials["api_token"]; !ok {
		t.Errorf("Credentials: got %v, want an api_token key", v.Credentials)
	}
}

func TestNewJobEntryDefaults(t *testing.T) {
	v := NewJobEntryDefaults()
	if v.JobType != "" {
		t.Errorf("JobType: got %q, want empty", v.JobType)
	}
	if v.Interval.Duration != time.Hour {
		t.Errorf("Interval: got %v, want 1h", v.Interval)
	}
	if v.Activated != false {
		t.Errorf("Activated: got %v, want false", v.Activated)
	}
}

func TestNewDefaultConfigServerHTTP2Defaults(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Server.WriteByteTimeout.Duration != 5*time.Second {
		t.Errorf("Server.WriteByteTimeout: got %v, want 5s", cfg.Server.WriteByteTimeout.Duration)
	}
	if cfg.Server.MaxConcurrentStreams != 32 {
		t.Errorf("Server.MaxConcurrentStreams: got %d, want 32", cfg.Server.MaxConcurrentStreams)
	}
}

func TestNewTokenBucketEntryDefaults(t *testing.T) {
	entry := NewTokenBucketEntryDefaults()

	if entry.Limit.Duration != 2*time.Second {
		t.Errorf("Limit = %v, want 2s", entry.Limit.Duration)
	}
	if entry.Burst != 5 {
		t.Errorf("Burst = %d, want 5", entry.Burst)
	}
}

func TestNewFixedWindowEntryDefaults(t *testing.T) {
	entry := NewFixedWindowEntryDefaults()

	if entry.Prefix != "CHANGE_ME" {
		t.Errorf("Prefix = %q, want CHANGE_ME", entry.Prefix)
	}
	if entry.Window.Duration != time.Hour {
		t.Errorf("Window = %v, want 1h", entry.Window.Duration)
	}
	if entry.Max != 1 {
		t.Errorf("Max = %d, want 1", entry.Max)
	}
}

func TestNewLimitersDefaults(t *testing.T) {
	limiters := NewLimitersDefaults()

	entry, ok := limiters.TokenBucket[LimiterPasswordHashing]
	if !ok {
		t.Fatal("TokenBucket does not contain password_hashing")
	}
	if entry.Limit.Duration != 2*time.Second {
		t.Errorf("password_hashing limit = %v, want 2s", entry.Limit.Duration)
	}
	if entry.Burst != 5 {
		t.Errorf("password_hashing burst = %d, want 5", entry.Burst)
	}

	fixedWindowSeeds := []struct {
		label  string
		prefix string
		window time.Duration
		max    int64
	}{
		{"password_reset", "pr_", 1 * time.Hour, 1},
		{"email_change", "ec_", 1 * time.Hour, 1},
		{"email_verification_otp", "ev_", 2 * time.Minute, 1},
		{"auth_attempts", "aa_", 1 * time.Minute, 3},
	}
	for _, tc := range fixedWindowSeeds {
		seed, ok := limiters.FixedWindow[tc.label]
		if !ok {
			t.Fatalf("FixedWindow does not contain %s", tc.label)
		}
		if seed.Prefix != tc.prefix {
			t.Errorf("%s prefix = %q, want %q", tc.label, seed.Prefix, tc.prefix)
		}
		if seed.Window.Duration != tc.window {
			t.Errorf("%s window = %v, want %v", tc.label, seed.Window.Duration, tc.window)
		}
		if seed.Max != tc.max {
			t.Errorf("%s max = %d, want %d", tc.label, seed.Max, tc.max)
		}
	}
}

func TestNewDefaultConfigSeedsPasswordHashing(t *testing.T) {
	cfg := NewDefaultConfig()

	if _, ok := cfg.Limiter.TokenBucket[LimiterPasswordHashing]; !ok {
		t.Fatal("NewDefaultConfig().Limiter.TokenBucket does not contain password_hashing")
	}
}
