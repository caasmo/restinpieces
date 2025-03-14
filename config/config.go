package config

import (
	"time"
)

const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"
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
