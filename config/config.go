package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

const (
	EnvGoogleClientID     = "OAUTH2_GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "OAUTH2_GOOGLE_CLIENT_SECRET"
	EnvGithubClientID     = "OAUTH2_GITHUB_CLIENT_ID"
	EnvGithubClientSecret = "OAUTH2_GITHUB_CLIENT_SECRET"
)

type EnvVar struct {
	Name  string
	Value string
}

const (
	OAuth2ProviderGoogle = "google"
	OAuth2ProviderGitHub = "github"
)

type OAuth2Provider struct {
	Name         string
	ClientID     EnvVar
	ClientSecret EnvVar
	DisplayName  string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	PKCE         bool
}

func (c *OAuth2Provider) FillEnvVars() error {
	c.ClientID.Value = os.Getenv(c.ClientID.Name)
	c.ClientSecret.Value = os.Getenv(c.ClientSecret.Name)

	if c.ClientID.Value == "" || c.ClientSecret.Value == "" {
		return fmt.Errorf("missing environment variables for %s: %s and %s must be set",
			c.Name, c.ClientID.Name, c.ClientSecret.Name)
	}
	return nil
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
	// Addr is the HTTP server address to listen on (e.g. ":8080")
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

type Config struct {
	JwtSecret        []byte
	TokenDuration    time.Duration
	DBFile           string
	Scheduler        Scheduler
	Server           Server

	OAuth2Providers map[string]OAuth2Provider
}


const (
	DefaultReadTimeout       = 2 * time.Second
	DefaultReadHeaderTimeout = 2 * time.Second 
	DefaultWriteTimeout      = 3 * time.Second
	DefaultIdleTimeout       = 1 * time.Minute
	DefaultShutdownTimeout   = 15 * time.Second
)

//
func FillServer() Server {
	return Server{
		Addr:                   ":8080",
		ShutdownGracefulTimeout: DefaultShutdownTimeout,
		ReadTimeout:            DefaultReadTimeout,
		ReadHeaderTimeout:      DefaultReadHeaderTimeout,
		WriteTimeout:           DefaultWriteTimeout,
		IdleTimeout:            DefaultIdleTimeout,
	}
}

func Load(dbfile string) (*Config, error) {


	cfg := &Config{
		JwtSecret:       []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
		TokenDuration:   15 * time.Minute,
		DBFile:          dbfile,
		Scheduler: Scheduler{
			Interval:             15 * time.Second,
			MaxJobsPerTick:       100,
			ConcurrencyMultiplier: 2, // Default to 2x CPU cores
		},
		OAuth2Providers: make(map[string]OAuth2Provider),
	}

cfg.Server = FillServer(cfg)

	// Configure Google OAuth2 provider
	googleConfig := OAuth2Provider{
		Name:         OAuth2ProviderGoogle,
		ClientID:     EnvVar{Name: EnvGoogleClientID},
		ClientSecret: EnvVar{Name: EnvGoogleClientSecret},
		DisplayName:  "Google",
		RedirectURL:  "http://localhost:8080/oauth2/callback/",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		PKCE: true,
	}
	if err := googleConfig.FillEnvVars(); err != nil {
		slog.Warn("skipping Google OAuth2 provider", "error", err)
	} else {
		cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleConfig
	}

	// Configure GitHub OAuth2 provider
	githubConfig := OAuth2Provider{
		Name:         OAuth2ProviderGitHub,
		ClientID:     EnvVar{Name: EnvGithubClientID},
		ClientSecret: EnvVar{Name: EnvGithubClientSecret},
		DisplayName:  "GitHub",
		RedirectURL:  "http://localhost:8080/oauth2/callback/",
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
		PKCE: true,
	}
	if err := githubConfig.FillEnvVars(); err != nil {
		slog.Warn("skipping GitHub OAuth2 provider", "error", err)
	} else {
		cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubConfig
	}

	return cfg, nil
}
