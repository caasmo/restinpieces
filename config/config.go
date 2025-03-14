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

type Env struct {
	Name  string
	Value string
}

//type Envs []Env
//
//func NewEnvs() Envs {
//	return Envs{
//		{Name: EnvGoogleClientID, Value: ""},
//		{Name: EnvGoogleClientSecret, Value: ""},
//		{Name: EnvGithubClientID, Value: ""},
//		{Name: EnvGithubClientSecret, Value: ""},
//	}
//}

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
