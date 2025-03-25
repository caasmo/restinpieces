package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"
)

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

type Jwt struct {
	AuthSecret                     []byte
	AuthTokenDuration              time.Duration
	VerificationEmailSecret        []byte
	VerificationEmailTokenDuration time.Duration
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

type EndpointsConfig struct {
	AuthRefresh              string `json:"auth_refresh"`
	AuthWithPassword         string `json:"auth_with_password"`
	AuthWithOAuth2           string `json:"auth_with_oauth2"`
	RequestVerification      string `json:"request_verification"`
	RegisterWithPassword     string `json:"register_with_password"`
	ListOAuth2Providers      string `json:"list_oauth2_providers"`
	ConfirmVerification      string `json:"confirm_verification"`
}

// DefaultEndpoints returns the standard endpoint paths
func DefaultEndpoints() EndpointsConfig {
	return EndpointsConfig{
		AuthRefresh:          "POST /api/auth-refresh",
		AuthWithPassword:     "POST /api/auth-with-password", 
		AuthWithOAuth2:       "POST /api/auth-with-oauth2",
		RequestVerification:  "POST /api/request-verification",
		RegisterWithPassword: "POST /api/register-with-password",
		ListOAuth2Providers:  "GET /api/list-oauth2-providers",
		ConfirmVerification:  "POST /api/confirm-verification",
	}
}

type Config struct {
	Jwt             Jwt
	DBFile          string
	Scheduler       Scheduler
	Server          Server
	OAuth2Providers map[string]OAuth2Provider
	Smtp            Smtp
	PublicDir       string // Directory to serve static files from
	Endpoints       EndpointsConfig
}

const (
	DefaultReadTimeout       = 2 * time.Second
	DefaultReadHeaderTimeout = 2 * time.Second
	DefaultWriteTimeout      = 3 * time.Second
	DefaultIdleTimeout       = 1 * time.Minute
	DefaultShutdownTimeout   = 15 * time.Second
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

func Load(dbfile string) (*Config, error) {

	cfg := &Config{
		Endpoints: DefaultEndpoints(),
		Jwt: Jwt{
			AuthSecret:                     []byte("test_auth_secret_32_bytes_long_xxxxxx"),
			AuthTokenDuration:              45 * time.Minute,
			VerificationEmailSecret:        []byte("test_verification_secret_32_bytes_xxxx"),
			VerificationEmailTokenDuration: 24 * time.Hour,
		},
		DBFile: dbfile,
		PublicDir: "test/html/public", // Default public directory
		Scheduler: Scheduler{
			Interval:              15 * time.Second,
			MaxJobsPerTick:        10,
			ConcurrencyMultiplier: 2, // Default to 2x CPU cores
		},
		OAuth2Providers: make(map[string]OAuth2Provider),
	}

	cfg.Server = FillServer(cfg)

	// Gmail SMTP configuration with detailed documentation
	gmailSmtp := Smtp{
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
		slog.Info("Gmail SMTP configuration loaded",
			"host", gmailSmtp.Host,
			"port", gmailSmtp.Port,
			"username", gmailSmtp.Username)
	} else {
		slog.Warn("Gmail SMTP configuration: Missing env variables. Skipping SMTP configuration")
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
	} else {
		slog.Warn("skipping Google OAuth2 provider - missing client ID or secret")
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
	} else {
		slog.Warn("skipping GitHub OAuth2 provider - missing client ID or secret")
	}

	return cfg, nil
}
