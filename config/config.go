package config

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

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
	DBPath          string                    `toml:"db_path" comment:"Path to SQLite database file"`
	PublicDir       string                    `toml:"public_dir" comment:"Directory containing static web assets"`
	Source          string                    `toml:"-" comment:"[READONLY] Source of config - 'file:<path>' or 'db'"`
	Jwt             Jwt                       `toml:"jwt" comment:"JSON Web Token settings"`
	Scheduler       Scheduler                 `toml:"scheduler" comment:"Background job scheduler settings"`
	Server          Server                    `toml:"server" comment:"HTTP server configuration"`
	RateLimits      RateLimits                `toml:"rate_limits" comment:"Rate limiting settings"`
	OAuth2Providers map[string]OAuth2Provider `toml:"oauth2_providers" comment:"OAuth2 provider configurations"`
	Smtp            Smtp                      `toml:"smtp" comment:"SMTP email settings"`
	Endpoints       Endpoints                 `toml:"endpoints" comment:"API endpoint paths"`
	Acme            Acme                      `toml:"acme" comment:"ACME/Let's Encrypt certificate settings"`
	BlockIp         BlockIp                   `toml:"block_ip" comment:"IP blocking settings"`
	Maintenance     Maintenance               `toml:"maintenance" comment:"Maintenance mode settings"`
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
	AuthSecret                     string   `toml:"auth_secret" comment:"Secret key for auth tokens (set via JWT_AUTH_SECRET)"`
	AuthTokenDuration              Duration `toml:"auth_token_duration" comment:"Duration auth tokens remain valid"`
	VerificationEmailSecret        string   `toml:"verification_email_secret" comment:"Secret key for email verification tokens"`
	VerificationEmailTokenDuration Duration `toml:"verification_email_token_duration" comment:"Duration email verification tokens remain valid"`
	PasswordResetSecret            string   `toml:"password_reset_secret" comment:"Secret key for password reset tokens"`
	PasswordResetTokenDuration     Duration `toml:"password_reset_token_duration" comment:"Duration password reset tokens remain valid"`
	EmailChangeSecret              string   `toml:"email_change_secret" comment:"Secret key for email change tokens"`
	EmailChangeTokenDuration       Duration `toml:"email_change_token_duration" comment:"Duration email change tokens remain valid"`
}

type Scheduler struct {
	Interval              Duration `toml:"interval" comment:"How often to check for pending jobs"`
	MaxJobsPerTick        int      `toml:"max_jobs_per_tick" comment:"Max jobs to process per scheduler run"`
	ConcurrencyMultiplier int      `toml:"concurrency_multiplier" comment:"Workers per CPU core (2-8 for I/O bound)"`
}

type Server struct {
	Addr                    string   `toml:"addr" comment:"HTTP listen address (e.g. ':8080')"`
	ShutdownGracefulTimeout Duration `toml:"shutdown_graceful_timeout" comment:"Max time to wait for graceful shutdown"`
	ReadTimeout             Duration `toml:"read_timeout" comment:"Max time to read full request"`
	ReadHeaderTimeout       Duration `toml:"read_header_timeout" comment:"Max time to read request headers"`
	WriteTimeout            Duration `toml:"write_timeout" comment:"Max time to write response"`
	IdleTimeout             Duration `toml:"idle_timeout" comment:"Max time for idle keep-alive connections"`
	ClientIpProxyHeader     string   `toml:"client_ip_proxy_header" comment:"Header to trust for client IP (e.g. 'X-Forwarded-For')"`
	EnableTLS               bool     `toml:"enable_tls" comment:"Enable HTTPS/TLS"`
	CertData                string   `toml:"cert_data" comment:"PEM-encoded TLS certificate (alternative to file)"`
	KeyData                 string   `toml:"key_data" comment:"PEM-encoded TLS private key (alternative to file)"`
	RedirectAddr            string   `toml:"redirect_addr" comment:"HTTP->HTTPS redirect address (e.g. ':80')"`
}

func (s *Server) BaseURL() string {
	scheme := "http"
	if s.EnableTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, sanitizeAddrEmptyHost(s.Addr))
}

type RateLimits struct {
	PasswordResetCooldown     Duration `toml:"password_reset_cooldown" comment:"Min time between password reset requests"`
	EmailVerificationCooldown Duration `toml:"email_verification_cooldown" comment:"Min time between email verification requests"`
	EmailChangeCooldown       Duration `toml:"email_change_cooldown" comment:"Min time between email change requests"`
}

type OAuth2Provider struct {
	Name            string   `toml:"name" comment:"Provider identifier (e.g. 'google')"`
	ClientID        string   `toml:"client_id" comment:"OAuth2 client ID (set via env)"`
	ClientSecret    string   `toml:"client_secret" comment:"OAuth2 client secret (set via env)"`
	DisplayName     string   `toml:"display_name" comment:"User-facing provider name"`
	RedirectURL     string   `toml:"redirect_url" comment:"Callback URL (leave empty for dynamic)"`
	RedirectURLPath string   `toml:"redirect_url_path" comment:"Callback URL path (e.g. '/oauth2/callback') - uses server host/port"`
	AuthURL         string   `toml:"auth_url" comment:"OAuth2 authorization endpoint"`
	TokenURL        string   `toml:"token_url" comment:"OAuth2 token endpoint"`
	UserInfoURL     string   `toml:"user_info_url" comment:"User info API endpoint"`
	Scopes          []string `toml:"scopes" comment:"Requested OAuth2 scopes"`
	PKCE            bool     `toml:"pkce" comment:"Enable PKCE flow"`
}

type Smtp struct {
	Enabled     bool   `toml:"enabled" comment:"Enable SMTP email sending"`
	Host        string `toml:"host" comment:"SMTP server hostname"`
	Port        int    `toml:"port" comment:"SMTP server port (587/465/25)"`
	Username    string `toml:"username" comment:"SMTP username (set via env)"`
	Password    string `toml:"password" comment:"SMTP password (set via env)"`
	FromName    string `toml:"from_name" comment:"Sender display name"`
	FromAddress string `toml:"from_address" comment:"Sender email address"`
	LocalName   string `toml:"local_name" comment:"HELO/EHLO domain name"`
	AuthMethod  string `toml:"auth_method" comment:"Auth method (plain/login/cram-md5)"`
	UseTLS      bool   `toml:"use_tls" comment:"Use direct TLS (port 465)"`
	UseStartTLS bool   `toml:"use_start_tls" comment:"Use STARTTLS (port 587)"`
}

type Endpoints struct {
	RefreshAuth              string `toml:"refresh_auth" comment:"Refresh auth token endpoint"`
	RequestEmailVerification string `toml:"request_email_verification" comment:"Request email verification endpoint"`
	ConfirmEmailVerification string `toml:"confirm_email_verification" comment:"Confirm email verification endpoint"`
	ListEndpoints            string `toml:"list_endpoints" comment:"List available endpoints"`
	AuthWithPassword         string `toml:"auth_with_password" comment:"Password authentication endpoint"`
	AuthWithOAuth2           string `toml:"auth_with_oauth2" comment:"OAuth2 authentication endpoint"`
	RegisterWithPassword     string `toml:"register_with_password" comment:"Password registration endpoint"`
	ListOAuth2Providers      string `toml:"list_oauth2_providers" comment:"List available OAuth2 providers"`
	RequestPasswordReset     string `toml:"request_password_reset" comment:"Request password reset endpoint"`
	ConfirmPasswordReset     string `toml:"confirm_password_reset" comment:"Confirm password reset endpoint"`
	RequestEmailChange       string `toml:"request_email_change" comment:"Request email change endpoint"`
	ConfirmEmailChange       string `toml:"confirm_email_change" comment:"Confirm email change endpoint"`
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
	Enabled                 bool     `toml:"enabled" comment:"Enable ACME certificate management"`
	Email                   string   `toml:"email" comment:"ACME account email"`
	Domains                 []string `toml:"domains" comment:"Domains for certificate"`
	DNSProvider             string   `toml:"dns_provider" comment:"DNS provider for challenges (e.g. 'cloudflare')"`
	RenewalDaysBeforeExpiry int      `toml:"renewal_days_before_expiry" comment:"Days before expiry to renew"`
	CloudflareApiToken      string   `toml:"cloudflare_api_token" comment:"Cloudflare API token (set via env)"`
	CADirectoryURL          string   `toml:"ca_directory_url" comment:"ACME directory URL"`
	AcmePrivateKey          string   `toml:"acme_private_key" comment:"ACME account private key (set via env)"`
}

// BlockIp holds configuration specific to IP blocking.
type BlockIp struct {
	Enabled bool `toml:"enabled" comment:"Enable automatic IP blocking"`
}

// Maintenance holds configuration for the maintenance mode feature.
type Maintenance struct {
	Enabled   bool `toml:"enabled" comment:"Enable maintenance mode feature"`
	Activated bool `toml:"activated" comment:"Currently in maintenance mode"`
}
