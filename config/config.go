package config

import (
	"time"
)

type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
	DBFile        string
	
	OAuth2GoogleClientID     string
	OAuth2GoogleClientSecret string
	OAuth2GithubClientID     string
	OAuth2GithubClientSecret string
	CallbackURL              string
}
