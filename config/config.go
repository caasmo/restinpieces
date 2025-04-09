package config

import (
	"fmt"
	"net"
	// "os" // No longer needed here
	"strings"
	"sync/atomic" // Added for atomic value
	"time"

	_ "embed"
	// "github.com/BurntSushi/toml" // No longer needed here
)

//go:embed config.toml
var DefaultConfigToml []byte

//go:embed .env.example
var DefaultEnvExample []byte

// Provider holds the application configuration and allows for atomic updates.
// LoadSecret loads a secret from an environment variable.
// If the env var is empty, it returns the defaultValue.
// Returns an error if both are empty.
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

type OAuth2Provider struct {
	Name         string
	ClientID     string
	ClientSecret string
	DisplayName  string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	PKCE         bool
}

type Scheduler struct {
	// Interval controls how often the scheduler checks for new jobs.
	// Should be set based on your job processing latency requirements - shorter
	// intervals provide faster job processing but increase database load.
	// Typical values range from 5 seconds to several minutes.
	Interval time.Duration

	// MaxJobsPerTick limits how many jobs are fetched from the database per schedule
	// interval. This prevents overwhelming the system when there are many pending jobs.
	// Set this based on your workers' processing capacity and job execution time.
	// For example, if jobs average 500ms to process and you have 10 workers, a value
	// of 20 would give a 2x buffer.
	MaxJobsPerTick int

	// ConcurrencyMultiplier determines how many concurrent workers are spawned per CPU core.
	// For CPU-bound jobs, keep this low (1-2). For I/O-bound jobs, higher values (2-8)
	// may improve throughput. Automatically scales with hardware resources.
	ConcurrencyMultiplier int
}

type Server struct {
	// Addr is the HTTP server address to listen on (e.g. ":8080" or "app.example.com:8080")
	Addr string

	// ShutdownGracefulTimeout is the maximum time to wait for graceful shutdown
	ShutdownGracefulTimeout time.Duration

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration

	// ReadHeaderTimeout is the maximum duration for reading request headers
	ReadHeaderTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration

	// ClientIpProxyHeader specifies which HTTP header to trust for client IP addresses
	// when behind a proxy (e.g. "X-Forwarded-For", "X-Real-IP"). Empty means use
	// the direct connection IP (r.RemoteAddr).
	ClientIpProxyHeader string

    // --- New TLS Fields ---
    EnableTLS       bool   // Default to false if not present
    CertFile        string // Path to TLS certificate file
    KeyFile         string // Path to TLS private key file

}

// BaseURL returns the full base URL including scheme and port
// Uses https in production (when not localhost)
// If Addr cannot be parsed, returns Addr as-is
func (s *Server) BaseURL() string {
	// Split host:port
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return s.Addr
	}

	// Default to localhost if no host specified
	if host == "" {
		host = "localhost"
	}

	// Determine scheme
	scheme := "https"
	if host == "localhost" {
		scheme = "http"
	}

	// Include port in URL
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

type RateLimits struct {
	// PasswordResetCooldown specifies how long a user must wait between
	// password reset requests to prevent abuse and email spam
	PasswordResetCooldown time.Duration

	// EmailVerificationCooldown specifies how long a user must wait between
	// email verification requests to prevent abuse and email spam
	EmailVerificationCooldown time.Duration

	// EmailChangeCooldown specifies how long a user must wait between
	// email change requests to prevent abuse and email spam
	EmailChangeCooldown time.Duration
}

type Jwt struct {
	AuthSecret                     string
	AuthTokenDuration              time.Duration
	VerificationEmailSecret        string
	VerificationEmailTokenDuration time.Duration
	PasswordResetSecret            string
	PasswordResetTokenDuration     time.Duration
	EmailChangeSecret              string
	EmailChangeTokenDuration       time.Duration
}

type Smtp struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromName    string // Sender name (e.g. "My App")
	FromAddress string // Sender email address (e.g. "noreply@example.com")
	LocalName   string // HELO/EHLO domain (empty defaults to "localhost")
	AuthMethod  string // "plain", "login", "cram-md5", or "none"
	UseTLS      bool   // Use explicit TLS
	UseStartTLS bool   // Use STARTTLS
}

type Endpoints struct {
	RefreshAuth              string `json:"refresh_auth"`
	RequestEmailVerification string `json:"request_email_verification"`
	ConfirmEmailVerification string `json:"confirm_email_verification"`
	ListEndpoints            string `json:"list_endpoints"`
	AuthWithPassword         string `json:"auth_with_password"`
	AuthWithOAuth2           string `json:"auth_with_oauth2"`
	RegisterWithPassword     string `json:"register_with_password"`
	ListOAuth2Providers      string `json:"list_oauth2_providers"`
	RequestPasswordReset     string `json:"request_password_reset"`
	ConfirmPasswordReset     string `json:"confirm_password_reset"`
	RequestEmailChange       string `json:"request_email_change"`
	ConfirmEmailChange       string `json:"confirm_email_change"`
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

type Config struct {
	Jwt             Jwt
	DBFile          string
	Scheduler       Scheduler
	Server          Server
	RateLimits      RateLimits
	OAuth2Providers map[string]OAuth2Provider
	Smtp            Smtp
	PublicDir       string // Directory to serve static files from
	Endpoints       Endpoints
	Proxy           Proxy
	Acme            Acme   // ACME/Let's Encrypt settings
	Source          string `toml:"-"` // [READONLY] Tracks config source - "file:<path>" or "db" (set internally, not loaded from config)
}

// Acme holds configuration for ACME (Let's Encrypt) certificate management.
type Acme struct {
	Enabled               bool          // Set to true to enable automatic certificate management
	Email                 string        // Email address for ACME account registration and notifications
	Domains               []string      // List of domains to include in the certificate
	DNSProvider           string        // DNS provider name (e.g., "cloudflare")
	RenewalDaysBeforeExpiry int           // Renew certificate if it expires within this many days
	CloudflareApiToken    string        `toml:"-"` // Cloudflare API Token (loaded from env)
	CADirectoryURL        string        // ACME directory URL (e.g., Let's Encrypt staging or production)

    // AcmePrivateKey is Primary: The private key is the fundamental identifier of the
    // Acme account. The email is just contact information associated with it. You
    // can even have multiple ACME accounts (each with its own unique private
    // key) registered under the same email address.
    //
    // Treat the acmePrivateKey as a vital, long-lived secret. Generate it
    // once, back it up securely, and provide it to your application via the
    // environment variable. Losing it means you'll need to start the ACME
    // registration process over with a new key. Generating a new key
    // frequently will likely break the renewal process due to rate limiting.
    // 
    // ex: openssl genpkey -algorithm RSA -outform PEM -pkeyopt rsa_keygen_bits:2048
	AcmePrivateKey        string        `toml:"-"` // ACME account private key PEM (loaded from env)
}

// BlockIp holds configuration specific to IP blocking.
type BlockIp struct {
	Enabled bool // Whether IP blocking is active
	// Add other blocking-related settings here (e.g., duration, thresholds)
}

// Proxy holds configuration for the proxy layer.
type Proxy struct {
	BlockIp BlockIp
}

// Load functions moved to config_load.go
