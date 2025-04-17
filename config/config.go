package config

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	_ "embed"
)

// variables used only by create-app
//
//go:embed config.toml.example
var TomlExample []byte

//go:embed .env.tmpl.example
var EnvTemplate []byte

// Provider holds the application configuration and allows for atomic updates.
type Provider struct {
	value atomic.Value // Holds the current *Config
}

// NewProvider creates a new configuration provider with the initial config.
// It panics if the initialConfig is nil.
func NewProvider(c *Config) *Provider {
	if c == nil {
		panic("initial config cannot be nil")
	}
	p := &Provider{}
	p.value.Store(c)
	return p
}

// Get returns the current configuration snapshot.
// It's safe for concurrent use.
func (p *Provider) Get() *Config {
	// Load returns interface{}, assert to *Config
	// This is safe because Store only accepts *Config.
	return p.value.Load().(*Config)
}

// Update atomically swaps the current configuration with the new one.
// The caller is responsible for ensuring newConfig is not nil.
func (p *Provider) Update(newConfig *Config) {
	// Assume newConfig is valid as the check is moved to the caller (signal handler)
	p.value.Store(newConfig)
	// Logging is now handled by the caller (e.g., signal handler in main.go)
}

const (
	OAuth2ProviderGoogle = "google"
	OAuth2ProviderGitHub = "github"
)

type Config struct {
	DBPath          string            `toml:"db_path"`
	PublicDir       string            `toml:"public_dir"`
	Source          string            `toml:"-"` // [READONLY] Tracks config source - "file:<path>" or "db" (set internally, not loaded from config)
	Jwt             Jwt               `toml:"jwt"`
	Scheduler       Scheduler         `toml:"scheduler"`
	Server          Server            `toml:"server"`
	RateLimits      RateLimits        `toml:"rate_limits"`
	OAuth2Providers map[string]OAuth2Provider `toml:"oauth2_providers"`
	Smtp            Smtp              `toml:"smtp"`
	Endpoints       Endpoints         `toml:"endpoints"`
	Acme            Acme              `toml:"acme"`
	BlockIp         BlockIp           `toml:"block_ip"`
	Maintenance     Maintenance       `toml:"maintenance"`
}

// Duration is a wrapper around time.Duration that supports TOML unmarshalling
// from a string value (e.g., "3h", "15m", "1h30m").
type Duration struct {
	time.Duration
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// This allows TOML libraries like pelletier/go-toml/v2 to unmarshal
// TOML string values directly into a Duration field.
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	if err != nil {
		// Provide more context in the error message
		return fmt.Errorf("failed to parse duration '%s': %w", string(text), err)
	}
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
// This is useful if you ever need to marshal the config back to TOML,
// ensuring durations are written as strings.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

type Jwt struct {
	AuthSecret                     string   `toml:"auth_secret"`
	AuthTokenDuration              Duration `toml:"auth_token_duration"`
	VerificationEmailSecret        string   `toml:"verification_email_secret"`
	VerificationEmailTokenDuration Duration `toml:"verification_email_token_duration"`
	PasswordResetSecret            string   `toml:"password_reset_secret"`
	PasswordResetTokenDuration     Duration `toml:"password_reset_token_duration"`
	EmailChangeSecret              string   `toml:"email_change_secret"`
	EmailChangeTokenDuration       Duration `toml:"email_change_token_duration"`
}

type Scheduler struct {
	Interval              Duration `toml:"interval"`
	MaxJobsPerTick        int      `toml:"max_jobs_per_tick"`
	ConcurrencyMultiplier int      `toml:"concurrency_multiplier"`
}

type Server struct {
	Addr                   string   `toml:"addr"`
	ShutdownGracefulTimeout Duration `toml:"shutdown_graceful_timeout"`
	ReadTimeout            Duration `toml:"read_timeout"`
	ReadHeaderTimeout      Duration `toml:"read_header_timeout"`
	WriteTimeout           Duration `toml:"write_timeout"`
	IdleTimeout            Duration `toml:"idle_timeout"`
	ClientIpProxyHeader    string   `toml:"client_ip_proxy_header"`
	EnableTLS              bool     `toml:"enable_tls"`
	CertFile               string   `toml:"cert_file"`
	KeyFile                string   `toml:"key_file"`
	CertData               string   `toml:"cert_data"`
	KeyData                string   `toml:"key_data"`
	RedirectAddr           string   `toml:"redirect_addr"`
}

func (s *Server) BaseURL() string {
	scheme := "http"
	if s.EnableTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, sanitizeAddrEmptyHost(s.Addr))
}

type RateLimits struct {
	PasswordResetCooldown      Duration `toml:"password_reset_cooldown"`
	EmailVerificationCooldown Duration `toml:"email_verification_cooldown"`
	EmailChangeCooldown       Duration `toml:"email_change_cooldown"`
}

type OAuth2Provider struct {
	Name         string   `toml:"name"`
	ClientID     string   `toml:"client_id"`
	ClientSecret string   `toml:"client_secret"`
	DisplayName  string   `toml:"display_name"`
	RedirectURL  string   `toml:"redirect_url"`
	AuthURL      string   `toml:"auth_url"`
	TokenURL     string   `toml:"token_url"`
	UserInfoURL  string   `toml:"user_info_url"`
	Scopes       []string `toml:"scopes"`
	PKCE         bool     `toml:"pkce"`
}

type Smtp struct {
	Enabled     bool   `toml:"enabled"`
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	Username    string `toml:"username"`
	Password    string `toml:"password"`
	FromName    string `toml:"from_name"`
	FromAddress string `toml:"from_address"`
	LocalName   string `toml:"local_name"`
	AuthMethod  string `toml:"auth_method"`
	UseTLS      bool   `toml:"use_tls"`
	UseStartTLS bool   `toml:"use_start_tls"`
}

type Endpoints struct {
	RefreshAuth              string `toml:"refresh_auth"`
	RequestEmailVerification string `toml:"request_email_verification"`
	ConfirmEmailVerification string `toml:"confirm_email_verification"`
	ListEndpoints            string `toml:"list_endpoints"`
	AuthWithPassword         string `toml:"auth_with_password"`
	AuthWithOAuth2           string `toml:"auth_with_oauth2"`
	RegisterWithPassword     string `toml:"register_with_password"`
	ListOAuth2Providers      string `toml:"list_oauth2_providers"`
	RequestPasswordReset     string `toml:"request_password_reset"`
	ConfirmPasswordReset     string `toml:"confirm_password_reset"`
	RequestEmailChange       string `toml:"request_email_change"`
	ConfirmEmailChange       string `toml:"confirm_email_change"`
}

// Path extracts just the path portion from an endpoint string (removes method prefix)
func (e Endpoints) Path(endpoint string) string {
	parts := strings.SplitN(endpoint, " ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return endpoint // fallback if no method prefix
}

// ConfirmHtml returns the HTML confirmation page path for an endpoint
// Follows naming convention: /api/confirm-X → /confirm-X.html
// This ensures consistency between API endpoints and their corresponding HTML pages
func (e Endpoints) ConfirmHtml(endpoint string) string {
	path := e.Path(endpoint)

	// Remove /api/ prefix if present
	path = strings.TrimPrefix(path, "/api")

	// Replace path with .html version
	return path + ".html"
}

// Acme holds configuration for ACME (Let's Encrypt) certificate management.
type Acme struct {
	Enabled                 bool     `toml:"enabled"`
	Email                   string   `toml:"email"`
	Domains                 []string `toml:"domains"`
	DNSProvider             string   `toml:"dns_provider"`
	RenewalDaysBeforeExpiry int      `toml:"renewal_days_before_expiry"`
	CloudflareApiToken      string   `toml:"cloudflare_api_token"`
	CADirectoryURL          string   `toml:"ca_directory_url"`
	AcmePrivateKey          string   `toml:"acme_private_key"`
}

// BlockIp holds configuration specific to IP blocking.
type BlockIp struct {
	Enabled bool `toml:"enabled"`
}

// Maintenance holds configuration for the maintenance mode feature.
type Maintenance struct {
	Enabled   bool `toml:"enabled"`   // Is the maintenance mode feature available?
	Activated bool `toml:"activated"` // Is maintenance mode currently active?
}
