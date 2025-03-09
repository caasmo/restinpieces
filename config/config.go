package config

import (
	"time"
)

type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
	DBFile        string
}
