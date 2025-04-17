package config

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"filippo.io/age"
	"github.com/pelletier/go-toml/v2" // TOML v2 parser

	"github.com/caasmo/restinpieces/db" // Adjust import path if necessary
)

// LoadFromDb loads configuration from the database using the provided DbConfig and age key file.
func LoadFromDb(db db.DbConfig, logger *slog.Logger, ageKeyPath string) (*Config, error) {
	logger.Info("loading configuration from database")
	encryptedData, err := db.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("config: failed to get from db: %w", err)
	}

	// Check if config is empty
	if len(encryptedData) == 0 {
		logger.Warn("no configuration found in database")
		return nil, fmt.Errorf("config: no configuration found in database")
	}

	// --- Decrypt Config ---
	// Use the provided ageKeyPath
	keyContent, err := os.ReadFile(ageKeyPath)
	if err != nil {
		logger.Error("failed to read age key file", "path", ageKeyPath, "error", err)
		return nil, fmt.Errorf("failed to read age key file '%s': %w", ageKeyPath, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))
	if err != nil {
		logger.Error("failed to parse age identities", "path", ageKeyPath, "error", err)
		return nil, fmt.Errorf("failed to parse age identities from key file '%s': %w", ageKeyPath, err)
	}
	if len(identities) == 0 {
		logger.Error("no age identities found in key file", "path", ageKeyPath)
		return nil, fmt.Errorf("no age identities found in key file '%s'", ageKeyPath)
	}

	// Zero out the raw key material as soon as identities are parsed
	for i := range keyContent {
		keyContent[i] = 0
	}

	encryptedDataReader := bytes.NewReader(encryptedData) // Use the byte slice directly
	decryptedDataReader, err := age.Decrypt(encryptedDataReader, identities...)

	// Make identities eligible for GC immediately after use
	identities = nil // Remove reference to the slice and underlying identity objects

	if err != nil {
		logger.Error("failed to decrypt configuration data", "error", err)
		return nil, fmt.Errorf("failed to decrypt configuration data: %w", err)
	}

	decryptedBytes, err := io.ReadAll(decryptedDataReader)
	if err != nil {
		logger.Error("failed to read decrypted data stream", "error", err)
		return nil, fmt.Errorf("failed to read decrypted data stream: %w", err)
	}

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
