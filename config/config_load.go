package config

import (
	"fmt"
	"log/slog"
	// Removed io, os, bytes, filippo.io/age imports as they are handled by SecureConfig

	"github.com/pelletier/go-toml/v2" // TOML v2 parser

	"github.com/caasmo/restinpieces/db" // Adjust import path if necessary
)

// LoadFromDb loads the main application configuration from the database.
// It uses a SecureConfig implementation (created internally using age) to handle decryption.
func LoadFromDb(dbCfg db.DbConfig, logger *slog.Logger, ageKeyPath string) (*Config, error) {
	logger.Info("initializing secure config loader (age)", "key_path", ageKeyPath)
	secureLoader, err := NewSecureConfigAge(dbCfg, ageKeyPath, logger)
	if err != nil {
		// Error already logged by NewSecureConfigAge
		return nil, fmt.Errorf("config: failed to initialize secure config loader: %w", err)
	}

	scope := db.ConfigScopeApplication
	logger.Info("loading application configuration via secure loader", "scope", scope)

	// --- Use SecureConfig Loader to get decrypted bytes ---
	decryptedBytes, err := secureLoader.Latest(scope)
	if err != nil {
		// Error should be logged by secureLoader.Latest
		// Wrap error for context
		return nil, fmt.Errorf("config: failed to load/decrypt config for scope '%s': %w", scope, err)
	}
	// Note: The check for empty data is now handled within secureLoader.Latest

	// --- Decryption is done, proceed with Unmarshal and Validate ---

	// --- Unmarshal TOML ---
	cfg := &Config{}
	err = toml.Unmarshal(decryptedBytes, cfg)
	if err != nil {
		logger.Error("failed to unmarshal TOML from database", "error", err)
		// Log the decrypted content only if unmarshalling fails, for debugging
		logger.Debug("decrypted content on unmarshal failure", "content", string(decryptedBytes))
		return nil, fmt.Errorf("config: failed to unmarshal TOML: %w", err)
	}

	// Validate the loaded configuration
	if err := Validate(cfg); err != nil {
		logger.Error("configuration validation failed after loading from DB", "error", err)
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	logger.Info("successfully loaded configuration from database")
	return cfg, nil
}
