package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic" // Added for atomic value
	"time"

	"github.com/BurntSushi/toml" 
	_ "embed"
)

//go:embed config.toml
var defaultConfigToml []byte

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

func LoadSecret(envVar string, defaultValue string) (string, error) {
	if value := os.Getenv(envVar); value != "" {
		return value, nil
	}
	if defaultValue != "" {
		return defaultValue, nil
	}
	return "", fmt.Errorf("secret required: set %s in environment variables or in config", envVar)
}

func LoadJwt(cfg *Config) error {
	var err error
	authSecret, err := LoadSecret("JWT_AUTH_SECRET", string(cfg.Jwt.AuthSecret))
	if err != nil {
		return fmt.Errorf("failed to load auth secret: %w", err)
	}
	cfg.Jwt.AuthSecret = []byte(authSecret)

	verifSecret, err := LoadSecret("JWT_VERIFICATION_EMAIL_SECRET", string(cfg.Jwt.VerificationEmailSecret))
	if err != nil {
		return fmt.Errorf("failed to load verification email secret: %w", err)
	}
	cfg.Jwt.VerificationEmailSecret = []byte(verifSecret)

	resetSecret, err := LoadSecret("JWT_PASSWORD_RESET_SECRET", string(cfg.Jwt.PasswordResetSecret))
	if err != nil {
		return fmt.Errorf("failed to load password reset secret: %w", err)
	}
	cfg.Jwt.PasswordResetSecret = []byte(resetSecret)

	changeSecret, err := LoadSecret("JWT_EMAIL_CHANGE_SECRET", string(cfg.Jwt.EmailChangeSecret))
	if err != nil {
		return fmt.Errorf("failed to load email change secret: %w", err)
	}
	cfg.Jwt.EmailChangeSecret = []byte(changeSecret)

	return nil
}

func LoadSmtp(cfg *Config) error {
	cfg.Smtp.Username = os.Getenv(EnvSmtpUsername)
	
	var err error
	cfg.Smtp.Password, err = LoadSecret(EnvSmtpPassword, cfg.Smtp.Password)
	if err != nil {
		return fmt.Errorf("failed to load SMTP password: %w", err)
	}

	if fromAddr := os.Getenv("SMTP_FROM_ADDRESS"); fromAddr != "" {
		cfg.Smtp.FromAddress = fromAddr
	}

	return nil
}

func LoadOAuth2(cfg *Config) error {
	baseURL := cfg.Server.BaseURL()

	// Google OAuth2
	if googleCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGoogle]; ok {
		var errID, errSecret error
		googleCfg.ClientID, errID = LoadSecret(EnvGoogleClientID, googleCfg.ClientID)
		googleCfg.ClientSecret, errSecret = LoadSecret(EnvGoogleClientSecret, googleCfg.ClientSecret)
		googleCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)
		
		if errID != nil || errSecret != nil {
			delete(cfg.OAuth2Providers, OAuth2ProviderGoogle)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleCfg
		}
	}

	// GitHub OAuth2
	if githubCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGitHub]; ok {
		var errID, errSecret error
		githubCfg.ClientID, errID = LoadSecret(EnvGithubClientID, githubCfg.ClientID)
		githubCfg.ClientSecret, errSecret = LoadSecret(EnvGithubClientSecret, githubCfg.ClientSecret)
		githubCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)
		
		if errID != nil || errSecret != nil {
			delete(cfg.OAuth2Providers, OAuth2ProviderGitHub)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubCfg
		}
	}

	return nil
}

func Load(dbfile string) (*Config, error) {
	cfg := &Config{}

	if _, err := toml.Decode(string(defaultConfigToml), cfg); err != nil {
		return nil, fmt.Errorf("failed to decode embedded default config: %w", err)
	}

	cfg.DBFile = dbfile

	if cfg.OAuth2Providers == nil {
		cfg.OAuth2Providers = make(map[string]OAuth2Provider)
	}

	if err := LoadJwt(cfg); err != nil {
		return nil, err
	}

	if err := LoadSmtp(cfg); err != nil {
		return nil, err
	}

	if err := LoadOAuth2(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
