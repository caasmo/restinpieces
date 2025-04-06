package config

import (
	"embed"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic" // Added for atomic value
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
	EnvGoogleClientID     = "OAUTH2_GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "OAUTH2_GOOGLE_CLIENT_SECRET"
	EnvGithubClientID     = "OAUTH2_GITHUB_CLIENT_ID"
	EnvGithubClientSecret = "OAUTH2_GITHUB_CLIENT_SECRET"
	EnvSmtpUsername       = "SMTP_USERNAME"
	EnvSmtpPassword       = "SMTP_PASSWORD"
)

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
	AuthSecret                     []byte
	AuthTokenDuration              time.Duration
	VerificationEmailSecret        []byte
	VerificationEmailTokenDuration time.Duration
	PasswordResetSecret            []byte
	PasswordResetTokenDuration     time.Duration
	EmailChangeSecret              []byte
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

// DefaultEndpoints returns the standard endpoint paths
func DefaultEndpoints() Endpoints {
	return Endpoints{
		RefreshAuth:              "POST /api/refresh-auth",
		RequestEmailVerification: "POST /api/request-email-verification",
		ConfirmEmailVerification: "POST /api/confirm-email-verification",
		ListEndpoints:            "GET /api/list-endpoints",
		AuthWithPassword:         "POST /api/auth-with-password",
		AuthWithOAuth2:           "POST /api/auth-with-oauth2",
		RegisterWithPassword:     "POST /api/register-with-password",
		ListOAuth2Providers:      "GET /api/list-oauth2-providers",
		RequestPasswordReset:     "POST /api/request-password-reset",
		ConfirmPasswordReset:     "POST /api/confirm-password-reset",
		RequestEmailChange:       "POST /api/request-email-change",
		ConfirmEmailChange:       "POST /api/confirm-email-change",
	}
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

const (
	DefaultReadTimeout         = 2 * time.Second
	DefaultReadHeaderTimeout   = 2 * time.Second
	DefaultWriteTimeout        = 3 * time.Second
	DefaultIdleTimeout         = 1 * time.Minute
	DefaultShutdownTimeout     = 15 * time.Second
	CodeOkEndpointsWithAuth    = "ok_endpoints_with_auth"
	CodeOkEndpointsWithoutAuth = "ok_endpoints_without_auth"
	MsgEndpointsWithAuth       = "List of all available endpoints"
	MsgEndpointsWithoutAuth    = "List of endpoints available without authentication"
)

func FillServer(cfg *Config) Server {
	s := cfg.Server

	if s.Addr == "" {
		s.Addr = ":8080"
	}
	if s.ShutdownGracefulTimeout == 0 {
		s.ShutdownGracefulTimeout = DefaultShutdownTimeout
	}
	if s.ReadTimeout == 0 {
		s.ReadTimeout = DefaultReadTimeout
	}
	if s.ReadHeaderTimeout == 0 {
		s.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}
	if s.WriteTimeout == 0 {
		s.WriteTimeout = DefaultWriteTimeout
	}
	if s.IdleTimeout == 0 {
		s.IdleTimeout = DefaultIdleTimeout
	}

	return s
}

//go:embed config.toml
var defaultConfigToml []byte

func Load(dbfile string) (*Config, error) {
	// 1. Start with an empty config struct
	cfg := &Config{}

	// 2. Load defaults from the embedded TOML file
	// We need to import a TOML parser, e.g., github.com/BurntSushi/toml
	// Ensure 'toml' is added to imports if not already present.
	// Let's assume 'toml' is imported for this block.
	if _, err := toml.Decode(string(defaultConfigToml), cfg); err != nil {
		return nil, fmt.Errorf("failed to decode embedded default config: %w", err)
	}

	// 3. Override specific fields based on runtime parameters or logic
	cfg.DBFile = dbfile // Set DBFile from argument

	// Ensure nested maps are initialized if needed (TOML decoder might handle this)
	if cfg.OAuth2Providers == nil {
		cfg.OAuth2Providers = make(map[string]OAuth2Provider)
	}

	// 4. Load secrets and provider-specific details from environment variables
	//    This overrides any placeholders potentially present in the embedded TOML.

	// Load JWT secrets (replace placeholders/defaults from TOML)
	// TODO: Define env vars for JWT secrets and load them here, e.g.:
	// cfg.Jwt.AuthSecret = []byte(os.Getenv("JWT_AUTH_SECRET"))
	// cfg.Jwt.VerificationEmailSecret = []byte(os.Getenv("JWT_VERIFICATION_EMAIL_SECRET"))
	// cfg.Jwt.PasswordResetSecret = []byte(os.Getenv("JWT_PASSWORD_RESET_SECRET"))
	// cfg.Jwt.EmailChangeSecret = []byte(os.Getenv("JWT_EMAIL_CHANGE_SECRET"))
	// Add error handling if secrets are missing in production.
	// For now, retain the test secrets if env vars are not set (consider removing in prod builds)
	if authSecret := os.Getenv("JWT_AUTH_SECRET"); authSecret != "" {
		cfg.Jwt.AuthSecret = []byte(authSecret)
	} else if len(cfg.Jwt.AuthSecret) == 0 { // Only set test secret if not set by TOML or ENV
		cfg.Jwt.AuthSecret = []byte("test_auth_secret_32_bytes_long_xxxxxx")
	}
	// Repeat for other JWT secrets...
	if verifSecret := os.Getenv("JWT_VERIFICATION_EMAIL_SECRET"); verifSecret != "" {
		cfg.Jwt.VerificationEmailSecret = []byte(verifSecret)
	} else if len(cfg.Jwt.VerificationEmailSecret) == 0 {
		cfg.Jwt.VerificationEmailSecret = []byte("test_verification_secret_32_bytes_xxxx")
	}
	if resetSecret := os.Getenv("JWT_PASSWORD_RESET_SECRET"); resetSecret != "" {
		cfg.Jwt.PasswordResetSecret = []byte(resetSecret)
	} else if len(cfg.Jwt.PasswordResetSecret) == 0 {
		cfg.Jwt.PasswordResetSecret = []byte("test_password_reset_secret_32_bytes_xxxx")
	}
	if changeSecret := os.Getenv("JWT_EMAIL_CHANGE_SECRET"); changeSecret != "" {
		cfg.Jwt.EmailChangeSecret = []byte(changeSecret)
	} else if len(cfg.Jwt.EmailChangeSecret) == 0 {
		cfg.Jwt.EmailChangeSecret = []byte("test_email_change_secret_32_bytes_xxxx")
	}


	// Load SMTP credentials (overrides TOML placeholders/defaults)
	cfg.Smtp.Username = os.Getenv(EnvSmtpUsername)
	cfg.Smtp.Password = os.Getenv(EnvSmtpPassword)
	if fromAddr := os.Getenv("SMTP_FROM_ADDRESS"); fromAddr != "" {
		cfg.Smtp.FromAddress = fromAddr
	}

	// Load OAuth2 credentials and update RedirectURLs (overrides TOML placeholders/defaults)
	baseURL := cfg.Server.BaseURL() // Calculate BaseURL after server defaults are potentially set by TOML

	// Google OAuth2
	if googleCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGoogle]; ok {
		googleCfg.ClientID = os.Getenv(EnvGoogleClientID)
		googleCfg.ClientSecret = os.Getenv(EnvGoogleClientSecret)
		googleCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL) // Update RedirectURL
		if googleCfg.ClientID != "" && googleCfg.ClientSecret != "" {
			cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleCfg
		} else {
			delete(cfg.OAuth2Providers, OAuth2ProviderGoogle) // Remove if secrets are missing
		}
	}

	// GitHub OAuth2
	if githubCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGitHub]; ok {
		githubCfg.ClientID = os.Getenv(EnvGithubClientID)
		githubCfg.ClientSecret = os.Getenv(EnvGithubClientSecret)
		githubCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL) // Update RedirectURL
		if githubCfg.ClientID != "" && githubCfg.ClientSecret != "" {
			cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubCfg
		} else {
			delete(cfg.OAuth2Providers, OAuth2ProviderGitHub) // Remove if secrets are missing
		}
	}

	// 5. Apply any final programmatic defaults or validations if needed
	//    (FillServer might be redundant now if TOML covers server defaults)
	// cfg.Server = FillServer(cfg) // Re-evaluate if this is needed

	return cfg, nil
}
		// Host is always smtp.gmail.com for Gmail
		Host: "smtp.gmail.com",

		// Port 587 is required for STARTTLS
		Port: 587,

		// Username must be the full Gmail address used for authentication
		// Example: "myaccount@gmail.com"
		// This must match the account where you've configured app passwords
		Username: os.Getenv(EnvSmtpUsername),

		// Password can be either:
		// 1. Your Gmail account password (not recommended)
		// 2. An app-specific password (recommended)
		// To create an app password:
		// 1. Enable 2FA on your Google account
		// 2. Go to Google Account > Security > App passwords
		// 3. Generate a password for "Mail" application
		Password: os.Getenv(EnvSmtpPassword),

		// FromName is the display name shown in email clients
		// Example: "My App" will show as "My App <noreply@example.com>"
		FromName: "My App",

		// FromAddress can be either:
		// 1. The same as Username (your Gmail address)
		// 2. A verified alias or Google Workspace email
		// To configure a different FromAddress:
		// 1. Go to Gmail Settings → Accounts and Import
		// 2. Under "Send mail as", click "Add another email address"
		// 3. Follow the verification steps
		FromAddress: os.Getenv("SMTP_FROM_ADDRESS"),

		// LocalName is the HELO/EHLO identifier sent to the SMTP server
		// In production, set this to your application's domain name
		// Example: "app.example.com"
		// This helps with email deliverability and prevents being flagged as spam
		// Note: Only supported by some SMTP providers like Gmail
		// If empty, defaults to "localhost" which should only be used in development
		LocalName: "",

		// AuthMethod must be "plain" for Gmail
		AuthMethod: "plain",

		// UseTLS should be false for port 587
		UseTLS: false,

		// UseStartTLS must be true for Gmail on port 587
		UseStartTLS: true,
	}

	// If Gmail credentials are detected, add to SMTP config
	if strings.HasSuffix(gmailSmtp.Username, "@gmail.com") && gmailSmtp.Password != "" {
		cfg.Smtp = gmailSmtp
	}

	// Configure Google OAuth2 provider
	googleConfig := OAuth2Provider{
		Name:         OAuth2ProviderGoogle,
		ClientID:     os.Getenv(EnvGoogleClientID),
		ClientSecret: os.Getenv(EnvGoogleClientSecret),
		DisplayName:  "Google",
		RedirectURL:  fmt.Sprintf("%s/oauth2/callback/", cfg.Server.BaseURL()),
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		PKCE: true,
	}
	if googleConfig.ClientID != "" && googleConfig.ClientSecret != "" {
		cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleConfig
	}

	// Configure GitHub OAuth2 provider
	githubConfig := OAuth2Provider{
		Name:         OAuth2ProviderGitHub,
		ClientID:     os.Getenv(EnvGithubClientID),
		ClientSecret: os.Getenv(EnvGithubClientSecret),
		DisplayName:  "GitHub",
		RedirectURL:  "http://localhost:8080/oauth2/callback/",
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
		PKCE:         true,
	}
	if githubConfig.ClientID != "" && githubConfig.ClientSecret != "" {
		cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubConfig
	}

	return cfg, nil
}
