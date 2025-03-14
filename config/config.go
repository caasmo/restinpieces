package config

import (
	"os"
	"time"
)

const (
	EnvGoogleClientID     = "OAUTH2_GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "OAUTH2_GOOGLE_CLIENT_SECRET"
	EnvGithubClientID     = "OAUTH2_GITHUB_CLIENT_ID"
	EnvGithubClientSecret = "OAUTH2_GITHUB_CLIENT_SECRET"
)

type Env struct {
	Name  string
	Value string
}

const (
	OAuth2ProviderGoogle = "google"
	OAuth2ProviderGitHub = "github"
)

type OAuth2ProviderConfig struct {
	Name         string
	ClientID     Env
	ClientSecret Env
	DisplayName  string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	PKCE         bool
}

func (c *OAuth2ProviderConfig) FillEnvVars() {
	c.ClientID.Value = os.Getenv(c.ClientID.Name)
	c.ClientSecret.Value = os.Getenv(c.ClientSecret.Name)
}

// hasEnvVars checks if both ClientID and ClientSecret have non-empty values
func (c *OAuth2ProviderConfig) hasEnvVars() bool {
	return c.ClientID.Value != "" && c.ClientSecret.Value != ""
}

type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
	DBFile        string

	OAuth2Providers map[string]OAuth2ProviderConfig
	CallbackURL     string
}
