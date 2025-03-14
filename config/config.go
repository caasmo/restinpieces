package config

import (
	"time"
)

const (
	EnvGoogleClientID     = "OAUTH2_GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "OAUTH2_GOOGLE_CLIENT_SECRET"
	EnvGithubClientID     = "OAUTH2_GITHUB_CLIENT_ID"
	EnvGithubClientSecret = "OAUTH2_GITHUB_CLIENT_SECRET"
)

type envOauth2 struct {
	Name  string
	Value string
}

type env struct {
	GoogleClientID     envOauth2
	GoogleClientSecret envOauth2
	GithubClientID     envOauth2
	GithubClientSecret envOauth2
}

const (
	OAuth2ProviderGoogle = "google"
	OAuth2ProviderGitHub = "github"
)

type OAuth2ProviderConfig struct {
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

type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
	DBFile        string
	
	OAuth2Providers map[string]OAuth2ProviderConfig
	CallbackURL     string
}
