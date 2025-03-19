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

type OAuth2ProviderConfig struct {
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
	// UserInfoFields defines the mapping between provider-specific fields and our standard fields
	UserInfoFields map[string]string `json:"userInfoFields"`
}

func (c *OAuth2ProviderConfig) FillEnvVars() error {
	c.ClientID.Value = os.Getenv(c.ClientID.Name)
	c.ClientSecret.Value = os.Getenv(c.ClientSecret.Name)

	if c.ClientID.Value == "" || c.ClientSecret.Value == "" {
		return fmt.Errorf("missing environment variables for %s: %s and %s must be set",
			c.Name, c.ClientID.Name, c.ClientSecret.Name)
	}
	return nil
}

type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
	DBFile        string

	OAuth2Providers map[string]OAuth2ProviderConfig
}
