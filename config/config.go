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

// UserInfoFields defines mappings between provider-specific fields and our standard fields
// Standard OAuth2 user info field names
const (
	FieldID            = "id"
	FieldEmail         = "email"
	FieldName          = "name"
	FieldAvatar        = "avatar"
	FieldEmailVerified = "email_verified"
)

type UserInfoFields map[string]string

// Required returns a slice of required field names
func (f UserInfoFields) Required() []string {
	return []string{FieldID, FieldEmail}
}

// Optional returns a slice of optional field names
func (f UserInfoFields) Optional() []string {
	return []string{FieldName, FieldAvatar, FieldEmailVerified}
}

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
	// UserInfoFields defines the mapping between provider-specific fields and our standard fields
	UserInfoFields UserInfoFields `json:"userInfoFields"`
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

type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
	DBFile        string

	OAuth2Providers map[string]OAuth2Provider
}
