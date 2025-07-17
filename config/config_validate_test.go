package config

import (
	"regexp"
	"testing"
)

// newTestConfig creates a valid config for tests.
func newTestConfig() *Config {
	cfg := NewDefaultConfig()
	// Override secrets for deterministic tests
	cfg.Jwt.AuthSecret = "test_secret_1"
	cfg.Jwt.VerificationEmailSecret = "test_secret_2"
	cfg.Jwt.PasswordResetSecret = "test_secret_3"
	cfg.Jwt.EmailChangeSecret = "test_secret_4"
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

// TestValidate serves as an integration test to ensure that the main Validate function
// correctly calls all the individual validation sub-routines. It does this by
// creating a valid configuration and then, for each sub-validator, introducing
// a single, specific error to confirm that the corresponding validation logic is triggered.
func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid default config", func(t *testing.T) {
		cfg := newTestConfig()
		if err := Validate(cfg); err != nil {
			t.Fatalf("Validate() with default config failed: %v", err)
		}
	})

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
	valid := Jwt{AuthSecret: "a", AuthTokenDuration: Duration{Duration: 1}, VerificationEmailSecret: "b", VerificationEmailTokenDuration: Duration{Duration: 1}, PasswordResetSecret: "c", PasswordResetTokenDuration: Duration{Duration: 1}, EmailChangeSecret: "d", EmailChangeTokenDuration: Duration{Duration: 1}}
	if err := validateJwt(&valid); err != nil {
		t.Errorf("valid case failed: %v", err)
	}

	invalidCases := []Jwt{
		{VerificationEmailSecret: "b", PasswordResetSecret: "c", EmailChangeSecret: "d"},
		{AuthSecret: "a", PasswordResetSecret: "c", EmailChangeSecret: "d"},
		{AuthSecret: "a", VerificationEmailSecret: "b", EmailChangeSecret: "d"},
		{AuthSecret: "a", VerificationEmailSecret: "b", PasswordResetSecret: "c"},
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
