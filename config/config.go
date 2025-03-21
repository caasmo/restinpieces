package config

import (
	"fmt"
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

type Config struct {
	JwtSecret        []byte
	TokenDuration    time.Duration
	DBFile           string
	Scheduler        Scheduler

	OAuth2Providers map[string]OAuth2Provider
}
