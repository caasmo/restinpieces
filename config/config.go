package config

import (
	"fmt"
	"log/slog"
	"regexp"
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

// Config holds the application configuration.
//
// Configuration fields use specific naming conventions for their behavior:
// - Fields named "Activated" can be toggled dynamically via config reload
// - Fields named "Enabled" require a server restart to take effect
// - Other boolean fields typically require restart unless documented otherwise
type Config struct {
	DBPath           string                    `toml:"db_path" comment:"Path to SQLite database file"`
	PublicDir        string                    `toml:"public_dir" comment:"Directory containing static web assets"`
	Source           string                    `toml:"-" comment:"[READONLY] Source of config - 'file:<path>' or 'db'"`
	Jwt              Jwt                       `toml:"jwt" comment:"JSON Web Token settings"`
	Scheduler        Scheduler                 `toml:"scheduler" comment:"Background job scheduler settings"`
	Server           Server                    `toml:"server" comment:"HTTP server configuration"`
	RateLimits       RateLimits                `toml:"rate_limits" comment:"Rate limiting settings"`
	OAuth2Providers  map[string]OAuth2Provider `toml:"oauth2_providers" comment:"OAuth2 provider configurations"`
	Smtp             Smtp                      `toml:"smtp" comment:"SMTP email settings"`
	Endpoints        Endpoints                 `toml:"endpoints" comment:"API endpoint paths"`
	BlockIp          BlockIp                   `toml:"block_ip" comment:"IP blocking settings"`
	Maintenance      Maintenance               `toml:"maintenance" comment:"Maintenance mode settings"`
	BlockUaList      BlockUaList               `toml:"block_ua_list" comment:"User-Agent block list settings"`
	Notifier         Notifier                  `toml:"notifier"`
	Log              Log                       `toml:"log" comment:"Logging configuration"`
	BlockRequestBody BlockRequestBody          `toml:"block_request_body" comment:"Request body size limiting configuration"`
	Metrics          Metrics                   `toml:"metrics" comment:"Metrics collection configuration"`
}

// Log contains Default (Batch) log configuration
type Log struct {
	Request LogRequest  `toml:"request" comment:"HTTP request logging configuration"`
	Batch   BatchLogger `toml:"batch" comment:"Batch logging configuration"`
}

// LogRequest contains HTTP request logging configuration
type LogRequest struct {
	Activated bool             `toml:"activated" comment:"Activate HTTP request logging"`
	Limits    LogRequestLimits `toml:"limits" comment:"Maximum field lengths"`
}

// LogRequestLimits defines maximum lengths for request log fields
type LogRequestLimits struct {
	URILength       int `toml:"uri" comment:"Max URI length (path + query) (minimum 64)"`
	UserAgentLength int `toml:"user_agent" comment:"Max User-Agent length (minimum 32)"`
	RefererLength   int `toml:"referer" comment:"Max Referer length (minimum 64)"`
	RemoteIPLength  int `toml:"remote_ip" comment:"Max IP address length (minimum 15). IPv4 max=15, IPv6 max=45 chars. 64 allows for ports/proxy info while preventing log injection"`
}

// BatchLogger contains batch logging configuration
type BatchLogger struct {
	FlushSize     int      `toml:"flush_size" comment:"Records to batch before writing"`
	ChanSize      int      `toml:"chan_size" comment:"Log record channel buffer size"`
	FlushInterval Duration `toml:"flush_interval" comment:"Max time between flushes"`
	Level         LogLevel `toml:"level" comment:"Minimum log level (debug, info, warn, error)"`
	DbPath        string   `toml:"db_path" comment:"SQLite database path for logs"`
}

// LogLevel is a wrapper around slog.Level that supports TOML unmarshalling
// Valid TOML values:
//   - String values (case insensitive): "debug", "info", "warn", "error"
//   - Numeric values: -4 (debug), 0 (info), 4 (warn), 8 (error)
//
// Example:
//
//	level = "debug"  # string name
//	level = "DEBUG"  # any case
//	level = -4       # numeric value
type LogLevel struct {
	slog.Level
}

// UnmarshalText implements the encoding.TextUnmarshaler interface
// Supports both string names and numeric values for log levels
func (l *LogLevel) UnmarshalText(text []byte) error {
	var level slog.Level
	if err := level.UnmarshalText(text); err != nil {
		return fmt.Errorf("invalid log level '%s': %w", string(text), err)
	}
	l.Level = level
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface
// Always marshals to the string name (e.g. "debug", "info")
func (l LogLevel) MarshalText() ([]byte, error) {
	return l.Level.MarshalText()
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

// Regexp is a wrapper around *regexp.Regexp that supports TOML unmarshalling
// from a string value directly into a compiled regular expression.
type Regexp struct {
	*regexp.Regexp
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (r *Regexp) UnmarshalText(text []byte) error {
	var err error
	r.Regexp, err = regexp.Compile(string(text))
	if err != nil {
		return fmt.Errorf("failed to compile regex '%s': %w", string(text), err)
	}
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
func (r Regexp) MarshalText() ([]byte, error) {
	if r.Regexp == nil {
		return []byte(""), nil
	}
	return []byte(r.Regexp.String()), nil
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

// Endpoints defines the structure for API endpoint paths.
// It includes TOML tags for configuration loading and JSON tags for API responses.
type Endpoints struct {
	RefreshAuth              string `toml:"refresh_auth" json:"refresh_auth" comment:"Refresh auth token endpoint"`
	RequestEmailVerification string `toml:"request_email_verification" json:"request_email_verification" comment:"Request email verification endpoint"`
	ConfirmEmailVerification string `toml:"confirm_email_verification" json:"confirm_email_verification" comment:"Confirm email verification endpoint"`
	ListEndpoints            string `toml:"list_endpoints" json:"list_endpoints" comment:"List available endpoints"`
	AuthWithPassword         string `toml:"auth_with_password" json:"auth_with_password" comment:"Password authentication endpoint"`
	AuthWithOAuth2           string `toml:"auth_with_oauth2" json:"auth_with_oauth2" comment:"OAuth2 authentication endpoint"`
	RegisterWithPassword     string `toml:"register_with_password" json:"register_with_password" comment:"Password registration endpoint"`
	ListOAuth2Providers      string `toml:"list_oauth2_providers" json:"list_oauth2_providers" comment:"List available OAuth2 providers"`
	RequestPasswordReset     string `toml:"request_password_reset" json:"request_password_reset" comment:"Request password reset endpoint"`
	ConfirmPasswordReset     string `toml:"confirm_password_reset" json:"confirm_password_reset" comment:"Confirm password reset endpoint"`
	RequestEmailChange       string `toml:"request_email_change" json:"request_email_change" comment:"Request email change endpoint"`
	ConfirmEmailChange       string `toml:"confirm_email_change" json:"confirm_email_change" comment:"Confirm email change endpoint"`
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

// BlockIp holds configuration specific to IP blocking.
type BlockIp struct {
	Enabled   bool `toml:"enabled" comment:"Enable automatic IP blocking (requires restart)"`
	Activated bool `toml:"activated" comment:"Activate IP blocking (can be toggled via config reload)"`
}

// Maintenance holds configuration for the maintenance mode feature.
type Maintenance struct {
	Activated bool `toml:"activated" comment:"Currently in maintenance mode"`
}

// BlockUaList holds configuration for blocking requests based on User-Agent patterns.
type BlockUaList struct {
	Activated bool `toml:"activated" comment:"Activate User-Agent block list"`
	// List holds a compiled regular expression for matching User-Agent strings.
	// RE2 Syntax Notes: Go uses the RE2 regex engine. For literal matching:
	// - Metacharacters like '.' MUST be escaped (e.g., `\.`).
	// - Characters like '-' or ' ' outside character classes `[]` are literal
	//   and do NOT require escaping, though RE2 tolerates unnecessary escapes (e.g., `\-`).
	// TOML Marshaling: When marshaling to TOML, the `go-toml` library might use
	// double quotes (`"..."`) with escaped backslashes (`\\`) or single quotes (`'...'`)
	// for literal strings. Both forms are correctly unmarshaled back into the
	// intended regex pattern string by the TOML parser before being compiled.
	// For manual TOML editing, use single quotes (`'...'`) for easier pasting of patterns.
	List Regexp `toml:"list" comment:"Regex for matching User-Agents to block"`
}

type Discord struct {
	Activated    bool     `toml:"activated" comment:"Activate the default Discord notifier"`
	WebhookURL   string   `toml:"webhook_url" comment:"Discord webhook URL"`
	APIRateLimit Duration `toml:"api_rate_limit" comment:"API call rate limit (e.g., '2s'). Discord webhooks generally allow ~30 requests/minute."`
	APIBurst     int      `toml:"api_burst" comment:"API call burst allowance (e.g., 1, 5)"`
	SendTimeout  Duration `toml:"send_timeout" comment:"Timeout for sending a single notification via Discord (e.g., '10s')"`
}

type Notifier struct {
	Discord Discord `toml:"discord" comment:"Default Discord notifier configuration"`
}

// BlockRequestBody holds configuration for request body size limiting
type Metrics struct {
	// Enabled controls whether metrics collection is compiled into the binary and available.
	// Changing this requires a server restart.
	Enabled bool `toml:"enabled" comment:"Enable metrics collection (requires restart)"`

	// Activated controls whether metrics are actively being collected.
	// This can be toggled via config reload without restart.
	Activated bool `toml:"activated" comment:"Activate metrics collection (can toggle via reload)"`

	// Endpoint is the HTTP path where metrics are exposed (e.g. "/metrics")
	Endpoint string `toml:"endpoint" comment:"HTTP path where metrics are exposed"`

	// AllowedIPs is a list of exact IP addresses that can access the metrics endpoint
	// Example: ["127.0.0.1", "192.168.1.100"]
	AllowedIPs []string `toml:"allowed_ips" comment:"List of exact IP addresses allowed to access metrics endpoint (no CIDR ranges)"`
}

type BlockRequestBody struct {
	// Activated enables/disables request body size limiting middleware
	Activated bool `toml:"activated" comment:"Enable request body size limiting"`

	// Limit is the maximum allowed request body size in bytes
	// Common values:
	// - 1MB (1048576) for typical APIs
	// - 10MB (10485760) for file uploads
	// - 100MB (104857600) for large media uploads
	Limit int64 `toml:"limit" comment:"Maximum allowed request body size in bytes"`

	// ExcludedPaths are URL paths that bypass size limiting
	// Path matching rules:
	// - Exact match required (case-sensitive)
	// - Trailing slashes are significant ('/path' ≠ '/path/')
	// - Query strings are ignored (matches path only)
	// - Paths should start with '/' (e.g. '/api/upload')
	// - No wildcards or pattern matching
	ExcludedPaths []string `toml:"excluded_paths" comment:"Paths that bypass size limiting"`
}
