package config

import (
	"fmt"
	"log/slog"

	"github.com/pelletier/go-toml/v2"
)

func Reload(secureCfg SecureConfig, provider *Provider, logger *slog.Logger) error {
	logger.Debug("Reload: Attempting to fetch latest configuration", "scope", ScopeApplication)
	decryptedBytes, err := secureCfg.Latest(ScopeApplication)
	if err != nil {
		logger.Error("Reload: Failed to fetch latest configuration", "scope", ScopeApplication, "error", err)
		return fmt.Errorf("failed to fetch latest configuration for scope %s: %w", ScopeApplication, err)
	}
	if len(decryptedBytes) == 0 {
		logger.Error("Reload: Fetched configuration is empty", "scope", ScopeApplication)
		return fmt.Errorf("fetched configuration for scope %s is empty", ScopeApplication)
	}
	logger.Debug("Reload: Successfully fetched new raw configuration", "scope", ScopeApplication, "size", len(decryptedBytes))

	newCfg := &Config{}
	logger.Debug("Reload: Unmarshalling new configuration", "scope", ScopeApplication)
	if err := toml.Unmarshal(decryptedBytes, newCfg); err != nil {
		logger.Error("Reload: Failed to unmarshal new configuration", "scope", ScopeApplication, "error", err)
		return fmt.Errorf("failed to unmarshal new configuration for scope %s: %w", ScopeApplication, err)
	}
	logger.Debug("Reload: Successfully unmarshalled new configuration", "scope", ScopeApplication)

	logger.Debug("Reload: Validating new configuration", "scope", ScopeApplication)
	if err := Validate(newCfg); err != nil {
		logger.Error("Reload: New configuration validation failed", "scope", ScopeApplication, "error", err)
		return fmt.Errorf("new configuration validation failed for scope %s: %w", ScopeApplication, err)
	}
	logger.Debug("Reload: New configuration validated successfully", "scope", ScopeApplication)

	newCfg.Source = "" // Clear source field, as it's now loaded from DB via secure store

	provider.Update(newCfg)
	logger.Info("Reload: Configuration successfully reloaded and updated in provider", "scope", ScopeApplication)

	return nil
}
