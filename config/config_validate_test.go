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
