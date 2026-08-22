package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// newTestCert generates a self-signed certificate and key, returning them as PEM-encoded strings.
func newTestCert(t *testing.T, notBefore, notAfter time.Time) (string, string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certOut := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("Unable to marshal private key: %v", err)
	}
	keyOut := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	return string(certOut), string(keyOut)
}

// newTestConfig creates a valid config for tests.
func newTestConfig() *Config {
	cfg := NewDefaultConfig()
	// Override secrets for deterministic tests
	cfg.Jwt.AuthSecret = "test_secret_1"
	cfg.Jwt.PasswordResetSecret = "test_secret_3"
	cfg.Jwt.EmailChangeOtpSecret = "test_secret_4"
	cfg.Jwt.VerificationEmailOtpSecret = "test_secret_5"
	cfg.Jwt.Oauth2StateSecret = "test_secret_6"
	cfg.Smtp.Enabled = true
	cfg.Smtp.Username = "user"
	cfg.Smtp.Password = "pass"
	cfg.Smtp.FromAddress = "from@example.com"
	cfg.Notifier.Discord.Activated = true
	cfg.Notifier.Discord.WebhookURL = "https://discord.com/api/webhooks/123/abc"
	// TLS tests are limited without real certs.
	// We disable it for the base valid config.
	cfg.Server.EnableTLS = false
	cfg.Server.CertData = ""
	cfg.Server.KeyData = ""
	return cfg
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid default config", func(t *testing.T) {
		cfg := newTestConfig()
		if err := Validate(cfg); err != nil {
			t.Fatalf("Validate() with default config failed: %v", err)
		}
	})

	// TestValidate serves as an integration test to ensure that the main Validate function
	// correctly calls all the individual validation sub-routines. It does this by
	// creating a valid configuration and then, for each sub-validator, introducing
	// a single, specific error to confirm that the corresponding validation logic is triggered.
	errorCases := []struct {
		name    string
		mutator func(*Config)
	}{
		{"invalid server", func(c *Config) { c.Server.Addr = "invalid" }},
		{"invalid jwt", func(c *Config) { c.Jwt.AuthSecret = "" }},
		{"invalid smtp", func(c *Config) { c.Smtp.Host = "" }},
		{"invalid oauth", func(c *Config) { c.OAuth2Providers["google"] = OAuth2Provider{} }},
		{"invalid block ua", func(c *Config) { c.BlockUaList.List.Regexp = nil }},
		{"invalid block host", func(c *Config) { c.BlockHost.AllowedHosts = []string{""} }},
		{"invalid notifier", func(c *Config) { c.Notifier.Discord.WebhookURL = "" }},
		{"invalid logger batch", func(c *Config) { c.Log.Batch.DbPath = "" }},
		{"invalid request log", func(c *Config) { c.Log.Request.Limits.URILength = 0 }},
		{"invalid block ip", func(c *Config) { c.BlockIp.Level = "" }},
		{"invalid cache", func(c *Config) { c.Cache.Level = "" }},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			tt.mutator(cfg)
			if err := Validate(cfg); err == nil {
				t.Errorf("Validate() expected an error for %s, but got nil", tt.name)
			}
		})
	}
}

func TestValidateCache(t *testing.T) {
	t.Parallel()
	validCases := []Cache{
		{Level: "small"},
		{Level: "medium"},
		{Level: "large"},
		{Level: "very-large"},
	}
	for _, cfg := range validCases {
		if err := validateCache(&cfg); err != nil {
			t.Errorf("validateCache(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []Cache{
		{Level: ""},
		{Level: "critical"},
		{Level: "small "},
	}
	for _, cfg := range invalidCases {
		if err := validateCache(&cfg); err == nil {
			t.Errorf("validateCache(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateBlockIp(t *testing.T) {
	t.Parallel()
	validCases := []BlockIp{
		{Enabled: false},
		{Enabled: true, Level: "low"},
		{Enabled: true, Level: "medium"},
		{Enabled: true, Level: "high"},
	}
	for _, cfg := range validCases {
		if err := validateBlockIp(&cfg); err != nil {
			t.Errorf("validateBlockIp(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []BlockIp{
		{Enabled: true, Level: ""},
		{Enabled: true, Level: "critical"},
	}
	for _, cfg := range invalidCases {
		if err := validateBlockIp(&cfg); err == nil {
			t.Errorf("validateBlockIp(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateLoggerBatch(t *testing.T) {
	t.Parallel()
	if err := validateLoggerBatch(&BatchLogger{ChanSize: 1, FlushSize: 1, FlushInterval: Duration{Duration: 1}, DbPath: "a"}); err != nil {
		t.Errorf("valid case failed: %v", err)
	}

	invalidCases := []BatchLogger{
		{ChanSize: 0, FlushSize: 1, FlushInterval: Duration{Duration: 1}, DbPath: "a"},
		{ChanSize: 1, FlushSize: 0, FlushInterval: Duration{Duration: 1}, DbPath: "a"},
		{ChanSize: 1, FlushSize: 1, FlushInterval: Duration{Duration: 0}, DbPath: "a"},
		{ChanSize: 1, FlushSize: 1, FlushInterval: Duration{Duration: 1}, DbPath: ""},
	}
	for _, cfg := range invalidCases {
		if err := validateLoggerBatch(&cfg); err == nil {
			t.Errorf("validateLoggerBatch(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateRequestLog(t *testing.T) {
	t.Parallel()
	validCfg := LogRequest{Activated: true, Limits: LogRequestLimits{URILength: 64, UserAgentLength: 32, RefererLength: 64, RemoteIPLength: 15}}
	if err := validateRequestLog(&validCfg); err != nil {
		t.Errorf("valid case failed: %v", err)
	}
	if err := validateRequestLog(&LogRequest{Activated: false}); err != nil {
		t.Errorf("disabled case failed: %v", err)
	}

	invalidCases := []LogRequest{
		{Activated: true, Limits: LogRequestLimits{URILength: 63, UserAgentLength: 32, RefererLength: 64, RemoteIPLength: 15}},
		{Activated: true, Limits: LogRequestLimits{URILength: 64, UserAgentLength: 31, RefererLength: 64, RemoteIPLength: 15}},
		{Activated: true, Limits: LogRequestLimits{URILength: 64, UserAgentLength: 32, RefererLength: 63, RemoteIPLength: 15}},
		{Activated: true, Limits: LogRequestLimits{URILength: 64, UserAgentLength: 32, RefererLength: 64, RemoteIPLength: 14}},
	}
	for _, cfg := range invalidCases {
		if err := validateRequestLog(&cfg); err == nil {
			t.Errorf("validateRequestLog(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateOAuth2Providers(t *testing.T) {
	t.Parallel()
	validCases := []map[string]OAuth2Provider{
		{"google": {RedirectURL: "/cb"}},
		{"google": {RedirectURLPath: "/cb"}},
	}
	for _, cfg := range validCases {
		if err := validateOAuth2Providers(cfg); err != nil {
			t.Errorf("validateOAuth2Providers(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []map[string]OAuth2Provider{
		{"google": {}},
		{"google": {RedirectURL: "/cb", UserInfoURL: "http://example.com"}},
	}
	for _, cfg := range invalidCases {
		if err := validateOAuth2Providers(cfg); err == nil {
			t.Errorf("validateOAuth2Providers(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateServer(t *testing.T) {
	t.Parallel()
	validCases := []Server{
		{Addr: ":8080"},
		{Addr: "localhost:8080"},
		{Addr: ":8080", RedirectAddr: ":80"},
	}
	for _, cfg := range validCases {
		if err := validateServer(&cfg); err != nil {
			t.Errorf("validateServer(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []Server{
		{},
		{Addr: "localhost"},
		{Addr: ":99999"},
		{Addr: ":8080", RedirectAddr: "localhost"},
		{Addr: ":8080", RedirectAddr: ":99999"}, // Invalid redirect port
		{Addr: ":8443", EnableTLS: true, KeyData: "key"},
		{Addr: ":8443", EnableTLS: true, CertData: "cert"},
		{Addr: ":8443", EnableTLS: true, CertData: "cert", KeyData: "key"}, // invalid cert data
	}
	for _, cfg := range invalidCases {
		if err := validateServer(&cfg); err == nil {
			t.Errorf("validateServer(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateJwt(t *testing.T) {
	t.Parallel()
	valid := Jwt{
		AuthSecret:                     "a",
		AuthTokenDuration:              Duration{Duration: 1},
		PasswordResetSecret:            "c",
		PasswordResetTokenDuration:     Duration{Duration: 1},
		EmailChangeOtpSecret:           "d",
		EmailChangeOtpTokenDuration:    Duration{Duration: 1},
		VerificationEmailOtpSecret:     "e",
		VerificationEmailOtpTokenDuration: Duration{Duration: 1},
		Oauth2StateSecret:              "f",
	}
	if err := validateJwt(&valid); err != nil {
		t.Errorf("valid case failed: %v", err)
	}

	invalidCases := []Jwt{
		{PasswordResetSecret: "c", EmailChangeOtpSecret: "d", VerificationEmailOtpSecret: "e"},
		{AuthSecret: "a", EmailChangeOtpSecret: "d", VerificationEmailOtpSecret: "e"},
		{AuthSecret: "a", PasswordResetSecret: "c", VerificationEmailOtpSecret: "e"},
		{AuthSecret: "a", PasswordResetSecret: "c", EmailChangeOtpSecret: "d"},
		{AuthSecret: "a", PasswordResetSecret: "c", EmailChangeOtpSecret: "d", VerificationEmailOtpSecret: "e", VerificationEmailOtpTokenDuration: Duration{Duration: 0}},
	}
	for _, cfg := range invalidCases {
		if err := validateJwt(&cfg); err == nil {
			t.Errorf("validateJwt() expected error, got nil")
		}
	}
}

func TestValidateSmtp(t *testing.T) {
	t.Parallel()
	valid := Smtp{Enabled: true, Host: "h", Port: 1, FromAddress: "f", Username: "u", Password: "p"}
	if err := validateSmtp(&valid); err != nil {
		t.Errorf("valid case failed: %v", err)
	}
	if err := validateSmtp(&Smtp{Enabled: false}); err != nil {
		t.Errorf("disabled case failed: %v", err)
	}

	invalidCases := []Smtp{
		{Enabled: true, Port: 1, FromAddress: "f", Username: "u", Password: "p"},
		{Enabled: true, Host: "h", FromAddress: "f", Username: "u", Password: "p"},
		{Enabled: true, Host: "h", Port: 1, Username: "u", Password: "p"},
		{Enabled: true, Host: "h", Port: 1, FromAddress: "f", Password: "p"},
		{Enabled: true, Host: "h", Port: 1, FromAddress: "f", Username: "u"},
	}
	for _, cfg := range invalidCases {
		if err := validateSmtp(&cfg); err == nil {
			t.Errorf("validateSmtp(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateBlockUaList(t *testing.T) {
	t.Parallel()
	valid := BlockUaList{Activated: true, List: Regexp{Regexp: regexp.MustCompile("a")}}
	if err := validateBlockUaList(&valid); err != nil {
		t.Errorf("valid case failed: %v", err)
	}
	if err := validateBlockUaList(&BlockUaList{Activated: false}); err != nil {
		t.Errorf("disabled case failed: %v", err)
	}

	invalid := BlockUaList{Activated: true, List: Regexp{}}
	if err := validateBlockUaList(&invalid); err == nil {
		t.Errorf("validateBlockUaList with nil regex expected error, got nil")
	}
}

func TestValidateBlockHost(t *testing.T) {
	t.Parallel()
	valid := BlockHost{Activated: true, AllowedHosts: []string{"a", "b"}}
	if err := validateBlockHost(&valid); err != nil {
		t.Errorf("valid case failed: %v", err)
	}
	if err := validateBlockHost(&BlockHost{Activated: false}); err != nil {
		t.Errorf("disabled case failed: %v", err)
	}

	invalidCases := []BlockHost{
		{Activated: true, AllowedHosts: []string{""}},
		{Activated: true, AllowedHosts: []string{"a b"}},
	}
	for _, cfg := range invalidCases {
		if err := validateBlockHost(&cfg); err == nil {
			t.Errorf("validateBlockHost(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateNotifier(t *testing.T) {
	t.Parallel()
	validCases := []Notifier{
		{Discord: Discord{Activated: false}},
		{Discord: Discord{Activated: true, WebhookURL: "https://discord.com/api/webhooks/1/2"}},
		{Discord: Discord{Activated: true, WebhookURL: "https://discordapp.com/api/webhooks/1/2"}},
	}
	for _, cfg := range validCases {
		if err := validateNotifier(&cfg); err != nil {
			t.Errorf("validateNotifier(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []Notifier{
		{Discord: Discord{Activated: true}},
		{Discord: Discord{Activated: true, WebhookURL: "https://example.com"}},
	}
	for _, cfg := range invalidCases {
		if err := validateNotifier(&cfg); err == nil {
			t.Errorf("validateNotifier(%+v) expected error, got nil", cfg)
		}
	}
}

func TestValidateServerTLS(t *testing.T) {
	t.Parallel()

	validCert, validKey := newTestCert(t, time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
	expiredCert, _ := newTestCert(t, time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour))
	futureCert, _ := newTestCert(t, time.Now().Add(1*time.Hour), time.Now().Add(2*time.Hour))

	testCases := []struct {
		name      string
		server    *Server
		expectErr bool
	}{
		{"TLS disabled", &Server{EnableTLS: false}, false},
		{"Valid TLS", &Server{EnableTLS: true, CertData: validCert, KeyData: validKey}, false},
		{"Missing CertData", &Server{EnableTLS: true, KeyData: validKey}, true},
		{"Missing KeyData", &Server{EnableTLS: true, CertData: validCert}, true},
		{"Invalid PEM block", &Server{EnableTLS: true, CertData: "not a pem block", KeyData: validKey}, true},
		{"Wrong PEM block type", &Server{EnableTLS: true, CertData: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("dummy")})), KeyData: validKey}, true},
		{"Expired certificate", &Server{EnableTLS: true, CertData: expiredCert, KeyData: validKey}, true},
		{"Not yet valid certificate", &Server{EnableTLS: true, CertData: futureCert, KeyData: validKey}, true},
		{"Invalid certificate bytes", &Server{EnableTLS: true, CertData: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("dummy")})), KeyData: validKey}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerTLS(tc.server)
			if (err != nil) != tc.expectErr {
				t.Fatalf("validateServerTLS() error = %v, expectErr %v", err, tc.expectErr)
			}
		})
	}
}

func TestValidateServerPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		portStr   string
		expectErr bool
	}{
		{"Valid port", "8080", false},
		{"Empty port", "", false},
		{"Port 0", "0", true},
		{"Port 65536", "65536", true},
		{"Non-numeric port", "http", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerPort(tc.portStr)
			if (err != nil) != tc.expectErr {
				t.Fatalf("validateServerPort() error = %v, expectErr %v", err, tc.expectErr)
			}
		})
	}
}

// backupLocalFixture returns a real backup directory and two real (empty)
// database files under a temp dir, for tests that require existing paths.
func backupLocalFixture(t *testing.T) (backupDir, appDB, otherDB string) {
	t.Helper()
	tempDir := t.TempDir()
	backupDir = filepath.Join(tempDir, "backups")
	if err := os.Mkdir(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	appDB = filepath.Join(tempDir, "app.db")
	otherDB = filepath.Join(tempDir, "other.db")
	for _, p := range []string{appDB, otherDB} {
		if err := os.WriteFile(p, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return backupDir, appDB, otherDB
}

func TestValidateBackup(t *testing.T) {
	t.Parallel()
	t.Run("online valid", func(t *testing.T) {
		backupDir, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"app-online": {SourcePath: appDB, DestPath: backupDir, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
	t.Run("vacuum valid", func(t *testing.T) {
		backupDir, appDB, _ := backupLocalFixture(t)
		b := &Backup{Vacuum: BackupVacuum{"app-vacuum": {SourcePath: appDB, DestPath: backupDir, Frequency: Duration{Duration: time.Hour}}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil for vacuum valid, got %v", err)
		}
	})
	t.Run("sqlite-rsync valid", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{SqliteRsync: BackupSqliteRsync{ListenAddr: "127.0.0.1:54321", Entries: map[string]BackupSqliteRsyncEntry{"app-rsync": {SourcePath: appDB, SyncTimeout: Duration{Duration: 15 * time.Minute}}}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil for rsync, got %v", err)
		}
	})
	t.Run("sqlite-rsync listen_addr empty rejected", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{SqliteRsync: BackupSqliteRsync{Entries: map[string]BackupSqliteRsyncEntry{"app-rsync": {SourcePath: appDB}}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for empty listen_addr")
		}
	})
	t.Run("sqlite-rsync listen_addr invalid", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{SqliteRsync: BackupSqliteRsync{ListenAddr: "bad", Entries: map[string]BackupSqliteRsyncEntry{"app-rsync": {SourcePath: appDB}}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for invalid listen_addr")
		}
	})
	t.Run("sqlite-rsync listen_addr valid", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{SqliteRsync: BackupSqliteRsync{ListenAddr: "127.0.0.1:54321", Entries: map[string]BackupSqliteRsyncEntry{"app-rsync": {SourcePath: appDB}}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil for valid listen_addr, got %v", err)
		}
	})
	t.Run("label with dot rejected", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"my.label": {SourcePath: appDB, DestPath: t.TempDir(), Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for label with dot")
		}
	})
	t.Run("label with space rejected", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"my label": {SourcePath: appDB, DestPath: t.TempDir(), Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for label with space")
		}
	})
	t.Run("label with dot rejected vacuum", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{Vacuum: BackupVacuum{"my.label": {SourcePath: appDB, DestPath: t.TempDir(), Frequency: Duration{Duration: time.Hour}}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for label with dot in vacuum")
		}
	})
	t.Run("label with dot rejected sqlite-rsync", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{SqliteRsync: BackupSqliteRsync{ListenAddr: "127.0.0.1:54321", Entries: map[string]BackupSqliteRsyncEntry{"my.label": {SourcePath: appDB}}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for label with dot in sqlite-rsync")
		}
	})
	t.Run("empty paths deactivate entry", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: appDB, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil for empty dest_path (deactivated), got: %v", err)
		}
	})
	t.Run("valid files mixed", func(t *testing.T) {
		backupDir, appDB, otherDB := backupLocalFixture(t)
		b := &Backup{
			Online: BackupOnline{"app": {SourcePath: appDB, DestPath: backupDir, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}},
			Vacuum: BackupVacuum{"other": {SourcePath: otherDB, DestPath: backupDir, Frequency: Duration{Duration: 30 * time.Minute}}},
			SqliteRsync: BackupSqliteRsync{ListenAddr: "127.0.0.1:54321", Entries: map[string]BackupSqliteRsyncEntry{"rsync": {SourcePath: appDB, SyncTimeout: Duration{Duration: 15 * time.Minute}}}},
		}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil for valid mixed config, got: %v", err)
		}
	})
	t.Run("no files configured validates ok", func(t *testing.T) {
		b := &Backup{}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("expected nil for no files configured, got: %v", err)
		}
	})
	t.Run("dest_path missing", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: appDB, DestPath: filepath.Join(t.TempDir(), "nope"), Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for missing dest_path, got nil")
		}
	})
	t.Run("dest_path is a file", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: appDB, DestPath: appDB, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for dest_path being a file, got nil")
		}
	})
	t.Run("source_path missing", func(t *testing.T) {
		backupDir, _, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: filepath.Join(t.TempDir(), "missing.db"), DestPath: backupDir, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for missing source_path, got nil")
		}
	})
	t.Run("source_path is a directory", func(t *testing.T) {
		backupDir, _, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: backupDir, DestPath: backupDir, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for source_path being a directory, got nil")
		}
	})
	t.Run("zero frequency", func(t *testing.T) {
		backupDir, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: appDB, DestPath: backupDir, Frequency: Duration{Duration: 0}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for zero frequency, got nil")
		}
	})
	t.Run("negative frequency", func(t *testing.T) {
		backupDir, appDB, _ := backupLocalFixture(t)
		b := &Backup{Online: BackupOnline{"db": {SourcePath: appDB, DestPath: backupDir, Frequency: Duration{Duration: -time.Hour}, PagesPerStep: 100}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for negative frequency, got nil")
		}
	})
	t.Run("duplicate basename allowed", func(t *testing.T) {
		backupDir, appDB, _ := backupLocalFixture(t)
		subDir := filepath.Join(filepath.Dir(appDB), "sub")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		otherAppDB := filepath.Join(subDir, "app.db")
		if err := os.WriteFile(otherAppDB, nil, 0644); err != nil {
			t.Fatal(err)
		}
		b := &Backup{
			Online: BackupOnline{
				"first":  {SourcePath: appDB, DestPath: backupDir, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100},
				"second": {SourcePath: otherAppDB, DestPath: backupDir, Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100},
			},
		}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("same basename with different map keys should be valid, got: %v", err)
		}
	})
	t.Run("negative pages_per_step", func(t *testing.T) {
		b := &Backup{Online: BackupOnline{"db": {SourcePath: "", DestPath: "", Frequency: Duration{Duration: time.Hour}, PagesPerStep: -1}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for negative pages_per_step, got nil")
		}
	})
	t.Run("zero pages_per_step allowed", func(t *testing.T) {
		b := &Backup{Online: BackupOnline{"db": {SourcePath: "", DestPath: "", Frequency: Duration{Duration: time.Hour}, PagesPerStep: 0}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("zero pages_per_step should be allowed (daemon default), got %v", err)
		}
	})
	t.Run("negative sleep interval", func(t *testing.T) {
		b := &Backup{Online: BackupOnline{"db": {SourcePath: "", DestPath: "", Frequency: Duration{Duration: time.Hour}, PagesPerStep: 100, SleepInterval: Duration{Duration: -time.Millisecond}}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for negative sleep interval, got nil")
		}
	})
	t.Run("negative sync_timeout", func(t *testing.T) {
		b := &Backup{SqliteRsync: BackupSqliteRsync{ListenAddr: "127.0.0.1:54321", Entries: map[string]BackupSqliteRsyncEntry{"db": {SourcePath: "", SyncTimeout: Duration{Duration: -time.Minute}}}}}
		if err := ValidateBackup(b); err == nil {
			t.Fatal("expected error for negative sync_timeout, got nil")
		}
	})
	t.Run("zero sync_timeout allowed", func(t *testing.T) {
		_, appDB, _ := backupLocalFixture(t)
		b := &Backup{SqliteRsync: BackupSqliteRsync{ListenAddr: "127.0.0.1:54321", Entries: map[string]BackupSqliteRsyncEntry{"db": {SourcePath: appDB, SyncTimeout: Duration{Duration: 0}}}}}
		if err := ValidateBackup(b); err != nil {
			t.Fatalf("zero sync_timeout should be allowed, got %v", err)
		}
	})
}
