package config

import (
	"fmt"
	"log/slog"

	"github.com/pelletier/go-toml/v2"
)

func Reload(secureCfg SecureConfig, provider *Provider, logger *slog.Logger) error {
	logger.Debug("Reload: Attempting to fetch latest application configuration")
	decryptedBytes, err := secureCfg.Latest(ScopeApplication)
	if err != nil {
		logger.Error("Reload: Failed to fetch latest application configuration", "error", err)
		return fmt.Errorf("failed to fetch latest application configuration: %w", err)
	}
	if len(decryptedBytes) == 0 {
		logger.Error("Reload: Fetched application configuration is empty")
		return fmt.Errorf("fetched application configuration is empty")
	}
	logger.Debug("Reload: Successfully fetched new raw application configuration", "size", len(decryptedBytes))

	newCfg := &Config{}
	logger.Debug("Reload: Unmarshalling new application configuration")
	if err := toml.Unmarshal(decryptedBytes, newCfg); err != nil {
		logger.Error("Reload: Failed to unmarshal new application configuration", "error", err)
		return fmt.Errorf("failed to unmarshal new application configuration: %w", err)
	}
	logger.Debug("Reload: Successfully unmarshalled new application configuration")

	logger.Debug("Reload: Validating new application configuration")
	if err := Validate(newCfg); err != nil {
		logger.Error("Reload: New application configuration validation failed", "error", err)
		return fmt.Errorf("new application configuration validation failed: %w", err)
	}
	logger.Debug("Reload: New application configuration validated successfully")

	newCfg.Source = "" // Clear source field, as it's now loaded from DB via secure store

	provider.Update(newCfg)
	logger.Info("Reload: Application configuration successfully reloaded and updated in provider")

	return nil
}
