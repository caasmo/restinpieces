package config

import (
	"fmt"
	"log/slog"
	// Removed io, os, bytes, filippo.io/age imports as they are handled by SecureConfig

	"github.com/pelletier/go-toml/v2" // TOML v2 parser

	"github.com/caasmo/restinpieces/db" // Adjust import path if necessary
)

// LoadFromDb loads the main application configuration from the database using SecureConfig.
func LoadFromDb(secureCfg SecureConfig) (*Config, error) {
	scope := db.ConfigScopeApplication
	
	// Get decrypted config content
	decryptedBytes, err := secureCfg.Latest(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to load/decrypt config: %w", err)
	}

	// Unmarshal TOML
	cfg := &Config{}
	if err := toml.Unmarshal(decryptedBytes, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
	}

	// Validate config
	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}
