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
	PublicDir        string                    `toml:"public_dir" comment:"Directory containing static web assets"`
	Source           string                    `toml:"-" comment:"[READONLY] Source of config - 'file:<path>' or 'db'"`
	Jwt              Jwt                       `toml:"jwt" comment:"JSON Web Token settings"`
	Scheduler        Scheduler                 `toml:"scheduler" comment:"Background job scheduler settings"`
	Server           Server                    `toml:"server" comment:"HTTP server configuration"`
	RateLimits       RateLimits                `toml:"rate_limits" comment:"Rate limiting settings"`
	OAuth2Providers  map[string]OAuth2Provider `toml:"oauth2_providers" comment:"OAuth2 provider configurations"`
	Smtp             Smtp                      `toml:"smtp" comment:"SMTP email settings"`
	Endpoints        Endpoints                 `toml:"endpoints" comment:"API endpoint paths"`
	Maintenance      Maintenance               `toml:"maintenance" comment:"Maintenance mode settings"`
	BlockIp          BlockIp                   `toml:"block_ip" comment:"IP blocking settings"`
	BlockUaList      BlockUaList               `toml:"block_ua_list" comment:"User-Agent block list settings"`
	BlockHost        BlockHost                 `toml:"block_host" comment:"Host blocking settings"`
	BlockRequestBody BlockRequestBody          `toml:"block_request_body" comment:"Request body size limiting configuration"`
	Notifier         Notifier                  `toml:"notifier"`
	Log              Log                       `toml:"log" comment:"Logging configuration"`
	Metrics          Metrics                   `toml:"metrics" comment:"Metrics collection configuration"`
	BackupLocal      BackupLocal               `toml:"backup_local" comment:"Local backup configuration"`
}

// BlockHost holds configuration for blocking requests based on the Host header.
type BlockHost struct {
	// Activated controls whether host blocking is currently active.
	Activated bool `toml:"activated" comment:"Activate host blocking"`
	// AllowedHosts is a list of hostnames that are allowed to access the server.
	// If the list is empty, all hosts are allowed.
	// Supports exact matches (e.g., "example.com") and wildcard subdomains (e.g., "*.example.com").
	AllowedHosts []string `toml:"allowed_hosts" comment:"List of allowed hostnames (e.g., 'example.com', '*.example.com')"`
}

// BackupLocal defines the settings for the local backup job.
type BackupLocal struct {
	SourcePath    string   `toml:"source_path" comment:"Path to the source database file to back up."`
	BackupDir     string   `toml:"backup_dir" comment:"Directory where backup files will be stored."`
	Strategy      string   `toml:"strategy" comment:"Backup strategy to use ('online' or 'vacuum'). 'online' is the default."`
	PagesPerStep  int      `toml:"pages_per_step" comment:"For 'online' strategy, the number of pages to copy in each step."`
	SleepInterval Duration `toml:"sleep_interval" comment:"For 'online' strategy, the duration to sleep between steps."`
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
	return []byte(d.String()), nil
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
	return []byte(r.String()), nil
}

type Jwt struct {
	// Secret key for auth tokens.
	AuthSecret string `toml:"auth_secret" comment:"Secret key for auth tokens"`
	// Duration for which standard authentication tokens are valid.
	AuthTokenDuration Duration `toml:"auth_token_duration" comment:"Duration auth tokens remain valid"`
	// Secret key for email verification tokens.
	VerificationEmailSecret string `toml:"verification_email_secret" comment:"Secret key for email verification tokens"`
	// Duration for which email verification tokens are valid.
	VerificationEmailTokenDuration Duration `toml:"verification_email_token_duration" comment:"Duration email verification tokens remain valid"`
	// Secret key for password reset tokens.
	PasswordResetSecret string `toml:"password_reset_secret" comment:"Secret key for password reset tokens"`
	// Duration for which password reset tokens are valid.
	PasswordResetTokenDuration Duration `toml:"password_reset_token_duration" comment:"Duration password reset tokens remain valid"`
	// Secret key for email change tokens.
	EmailChangeSecret string `toml:"email_change_secret" comment:"Secret key for email change tokens"`
	// Duration for which email change confirmation tokens are valid.
	EmailChangeTokenDuration Duration `toml:"email_change_token_duration" comment:"Duration email change tokens remain valid"`
}

// Scheduler defines settings for the background job processing queue.
type Scheduler struct {
	// Interval specifies how often the scheduler checks for pending jobs.
	Interval Duration `toml:"interval" comment:"How often to check for pending jobs"`
	// MaxJobsPerTick limits the number of jobs processed in a single scheduler run.
	MaxJobsPerTick int `toml:"max_jobs_per_tick" comment:"Max jobs to process per scheduler run"`
	// ConcurrencyMultiplier sets the number of concurrent workers per CPU core.
	// For I/O-bound tasks, a value between 2 and 8 is recommended.
	// For CPU-bound tasks, this should typically be 1.
	ConcurrencyMultiplier int `toml:"concurrency_multiplier" comment:"Workers per CPU core (2-8 for I/O bound)"`
}

type Server struct {
	// Network address and port the HTTP server listens on.
	// Examples: ":8080" (all interfaces, port 8080), "localhost:9000"
	Addr string `toml:"addr" comment:"HTTP listen address (e.g. ':8080')"`

	// Maximum duration the server waits for ongoing requests to complete before shutting down.
	ShutdownGracefulTimeout Duration `toml:"shutdown_graceful_timeout" comment:"Max time to wait for graceful shutdown"`

	// Maximum duration for reading the entire request, including the body.
	ReadTimeout Duration `toml:"read_timeout" comment:"Max time to read full request"`

	// Maximum duration for reading only the request headers.
	ReadHeaderTimeout Duration `toml:"read_header_timeout" comment:"Max time to read request headers"`

	// Maximum duration before timing out writes of the response.
	WriteTimeout Duration `toml:"write_timeout" comment:"Max time to write response"`

	// Maximum duration for waiting for the next request on a keep-alive connection.
	IdleTimeout Duration `toml:"idle_timeout" comment:"Max time for idle keep-alive connections"`

	// If behind a trusted proxy, specify the header containing the real client IP.
	// Common values: "X-Forwarded-For", "X-Real-IP". Leave empty if not behind a proxy.
	ClientIpProxyHeader string `toml:"client_ip_proxy_header" comment:"Header to trust for client IP (e.g. 'X-Forwarded-For')"`

	// Enable HTTPS/TLS for secure connections
	EnableTLS bool `toml:"enable_tls" comment:"Enable HTTPS/TLS"`

	// PEM-encoded TLS certificate data (alternative to cert_file)
	CertData string `toml:"cert_data" comment:"PEM-encoded TLS certificate (alternative to file)"`

	// PEM-encoded TLS private key data (alternative to key_file)
	KeyData string `toml:"key_data" comment:"PEM-encoded TLS private key (alternative to file)"`

	// Address for HTTP->HTTPS redirect server (e.g. ":80")
	// Only used when enable_tls is true
	RedirectAddr string `toml:"redirect_addr" comment:"HTTP->HTTPS redirect address (e.g. ':80')"`
}

func (s *Server) BaseURL() string {
	scheme := "http"
	if s.EnableTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, sanitizeAddrEmptyHost(s.Addr))
}

type RateLimits struct {
	// Minimum time a user must wait between requesting password resets for the same account.
	PasswordResetCooldown Duration `toml:"password_reset_cooldown" comment:"Min time between password reset requests"`
	// Minimum time a user must wait between requesting email verifications for the same email.
	EmailVerificationCooldown Duration `toml:"email_verification_cooldown" comment:"Min time between email verification requests"`
	// Minimum time a user must wait between requesting email address changes.
	EmailChangeCooldown Duration `toml:"email_change_cooldown" comment:"Min time between email change requests"`
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

// Smtp holds the configuration for sending emails via an SMTP server.
type Smtp struct {
	// Enabled controls whether the SMTP email sending functionality is active.
	Enabled bool `toml:"enabled" comment:"Enable SMTP email sending"`
	// Host is the SMTP server hostname or IP address.
	Host string `toml:"host" comment:"SMTP server hostname"`
	// Port is the SMTP server port. Common values are 587 (STARTTLS), 465 (TLS), or 25 (unencrypted).
	Port int `toml:"port" comment:"SMTP server port (587/465/25)"`
	// Username for SMTP authentication. It is recommended to set this via an environment variable.
	Username string `toml:"username" comment:"SMTP username (set via env)"`
	// Password for SMTP authentication. It is recommended to set this via an environment variable.
	Password string `toml:"password" comment:"SMTP password (set via env)"`
	// FromName is the display name for the sender (e.g., "My Application").
	FromName string `toml:"from_name" comment:"Sender display name"`
	// FromAddress is the email address from which emails are sent (e.g., "noreply@example.com").
	FromAddress string `toml:"from_address" comment:"Sender email address"`
	// LocalName is the domain name sent during the HELO/EHLO handshake. Defaults to "localhost".
	LocalName string `toml:"local_name" comment:"HELO/EHLO domain name"`
	// AuthMethod specifies the authentication mechanism, e.g., "plain", "login", "cram-md5".
	AuthMethod string `toml:"auth_method" comment:"Auth method (plain/login/cram-md5)"`
	// UseTLS enables a direct TLS connection (SMTPS), typically on port 465.
	UseTLS bool `toml:"use_tls" comment:"Use direct TLS (port 465)"`
	// UseStartTLS enables the STARTTLS command to upgrade an insecure connection to a secure one, typically on port 587.
	UseStartTLS bool `toml:"use_start_tls" comment:"Use STARTTLS (port 587)"`
}

// Endpoints defines the API endpoint paths for various authentication and account management actions.
// Each field maps a function to a specific HTTP method and path (e.g., "POST /api/refresh-auth").
type Endpoints struct {
	// RefreshAuth is the endpoint for refreshing an authentication token.
	RefreshAuth string `toml:"refresh_auth" json:"refresh_auth" comment:"Refresh auth token endpoint"`
	// RequestEmailVerification is the endpoint for users to request an email verification link.
	RequestEmailVerification string `toml:"request_email_verification" json:"request_email_verification" comment:"Request email verification endpoint"`
	// ConfirmEmailVerification is the endpoint for verifying a user's email address using a token.
	ConfirmEmailVerification string `toml:"confirm_email_verification" json:"confirm_email_verification" comment:"Confirm email verification endpoint"`
	// ListEndpoints is the endpoint that provides a list of all available API endpoints.
	ListEndpoints string `toml:"list_endpoints" json:"list_endpoints" comment:"List available endpoints"`
	// AuthWithPassword is the endpoint for authenticating a user with their email and password.
	AuthWithPassword string `toml:"auth_with_password" json:"auth_with_password" comment:"Password authentication endpoint"`
	// AuthWithOAuth2 is the endpoint for initiating or handling an OAuth2 authentication flow.
	AuthWithOAuth2 string `toml:"auth_with_oauth2" json:"auth_with_oauth2" comment:"OAuth2 authentication endpoint"`
	// RegisterWithPassword is the endpoint for creating a new user account with an email and password.
	RegisterWithPassword string `toml:"register_with_password" json:"register_with_password" comment:"Password registration endpoint"`
	// ListOAuth2Providers is the endpoint for listing all configured OAuth2 providers.
	ListOAuth2Providers string `toml:"list_oauth2_providers" json:"list_oauth2_providers" comment:"List available OAuth2 providers"`
	// RequestPasswordReset is the endpoint for users to request a password reset link.
	RequestPasswordReset string `toml:"request_password_reset" json:"request_password_reset" comment:"Request password reset endpoint"`
	// ConfirmPasswordReset is the endpoint for resetting a user's password using a token.
	ConfirmPasswordReset string `toml:"confirm_password_reset" json:"confirm_password_reset" comment:"Confirm password reset endpoint"`
	// RequestEmailChange is the endpoint for users to request an email address change.
	RequestEmailChange string `toml:"request_email_change" json:"request_email_change" comment:"Request email change endpoint"`
	// ConfirmEmailChange is the endpoint for confirming an email address change using a token.
	ConfirmEmailChange string `toml:"confirm_email_change" json:"confirm_email_change" comment:"Confirm email change endpoint"`
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
// This feature is intended to automatically block IP addresses based on
// certain criteria, such as excessive requests, to mitigate abuse.
type BlockIp struct {
	// Enabled determines if the IP blocking feature is compiled and available.
	// A restart is required to apply changes to this field.
	Enabled bool `toml:"enabled" comment:"Enable automatic IP blocking (requires restart)"`
	// Activated controls whether IP blocking is currently active.
	// This can be toggled dynamically via a configuration reload.
	Activated bool `toml:"activated" comment:"Activate IP blocking (can be toggled via config reload)"`
}

// Maintenance holds configuration for the maintenance mode feature.
// When activated, the application will serve a maintenance page or message
// for all incoming requests, effectively taking the service offline gracefully.
type Maintenance struct {
	// Activated controls whether maintenance mode is currently active.
	// This can be toggled dynamically via a configuration reload.
	Activated bool `toml:"activated" comment:"Currently in maintenance mode"`
}

// BlockUaList holds configuration for blocking requests based on User-Agent patterns.
// This is useful for filtering out bots, scrapers, or other unwanted clients.
type BlockUaList struct {
	// Activated controls whether the User-Agent block list is currently active.
	// This can be toggled dynamically via a configuration reload.
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

// Discord holds the configuration for sending notifications via a Discord webhook.
type Discord struct {
	// Activated controls whether the Discord notifier is active.
	Activated bool `toml:"activated" comment:"Activate the default Discord notifier"`
	// WebhookURL is the URL of the Discord webhook to which notifications will be sent.
	WebhookURL string `toml:"webhook_url" comment:"Discord webhook URL"`
	// APIRateLimit specifies the minimum time between API calls to avoid rate limiting.
	APIRateLimit Duration `toml:"api_rate_limit" comment:"API call rate limit (e.g., '2s'). Discord webhooks generally allow ~30 requests/minute."`
	// APIBurst allows for a certain number of requests to be made in quick succession before rate limiting is enforced.
	APIBurst int `toml:"api_burst" comment:"API call burst allowance (e.g., 1, 5)"`
	// SendTimeout is the maximum time to wait for a single notification to be sent to Discord.
	SendTimeout Duration `toml:"send_timeout" comment:"Timeout for sending a single notification via Discord (e.g., '10s')"`
}

// Notifier holds the configuration for various notification services.
// Currently, it only contains settings for Discord.
type Notifier struct {
	// Discord holds the configuration for the Discord notifier.
	Discord Discord `toml:"discord" comment:"Default Discord notifier configuration"`
}

// Metrics holds the configuration for collecting and exposing application metrics,
// typically for monitoring purposes (e.g., with Prometheus).
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

// BlockRequestBody holds configuration for limiting the size of incoming request bodies.
// This is a security measure to prevent denial-of-service attacks using large payloads.
type BlockRequestBody struct {
	// Activated enables or disables the request body size limiting middleware.
	// This can be toggled dynamically via a configuration reload.
	Activated bool `toml:"activated" comment:"Enable request body size limiting"`

	// Limit is the maximum allowed request body size in bytes.
	// Common values:
	// - 1MB (1048576) for typical APIs
	// - 10MB (10485760) for file uploads
	// - 100MB (104857600) for large media uploads
	Limit int64 `toml:"limit" comment:"Maximum allowed request body size in bytes"`

	// ExcludedPaths are URL paths that bypass the size limiting middleware.
	// Path matching rules:
	// - Exact match required (case-sensitive)
	// - Trailing slashes are significant ('/path' ≠ '/path/')
	// - Query strings are ignored (matches path only)
	// - Paths should start with '/' (e.g. '/api/upload')
	// - No wildcards or pattern matching
	ExcludedPaths []string `toml:"excluded_paths" comment:"Paths that bypass size limiting"`
}
