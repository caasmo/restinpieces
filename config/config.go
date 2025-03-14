package config

import (
	"time"
)

type OAuth2ProviderConfig struct {
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
	
	OAuth2Google OAuth2ProviderConfig
	OAuth2Github OAuth2ProviderConfig
	CallbackURL  string
}
