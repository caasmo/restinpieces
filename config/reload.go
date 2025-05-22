package config

import (
	"fmt"
	"log/slog"

	"strings"

	"github.com/pelletier/go-toml/v2"
)

func checkChangedRestartFields(oldCfg, newCfg *Config) []string {
	changed := []string{}

	if oldCfg.Server.Addr != newCfg.Server.Addr {
		changed = append(changed, "Server.Addr")
	}
	if oldCfg.Server.EnableTLS != newCfg.Server.EnableTLS {
		changed = append(changed, "Server.EnableTLS")
	}
	if oldCfg.Server.CertData != newCfg.Server.CertData {
		changed = append(changed, "Server.CertData")
	}
	if oldCfg.Server.KeyData != newCfg.Server.KeyData {
		changed = append(changed, "Server.KeyData")
	}
	if oldCfg.Server.RedirectAddr != newCfg.Server.RedirectAddr {
		changed = append(changed, "Server.RedirectAddr")
	}
	if oldCfg.DBPath != newCfg.DBPath {
		changed = append(changed, "DBPath")
	}

	return changed
}

// Reload returns a function that, when called, attempts to reload the application configuration.
// This allows the reload logic to be prepared once and executed later, typically on SIGHUP.
func Reload(configStore SecureStore, provider *Provider, logger *slog.Logger) func() error {
	// Return a closure that captures the necessary dependencies (configStore, provider, logger)
	return func() error {
		logger.Debug("Reload func: Attempting to fetch latest application configuration")
		decryptedBytes, _, err := configStore.Get(ScopeApplication, 0)
		if err != nil {
			logger.Error("Reload func: Failed to fetch latest application configuration", "error", err)
			return fmt.Errorf("failed to fetch latest application configuration: %w", err)
		}
		if len(decryptedBytes) == 0 {
			logger.Error("Reload func: Fetched application configuration is empty")
			return fmt.Errorf("fetched application configuration is empty")
		}
		logger.Debug("Reload func: Successfully fetched new raw application configuration", "size", len(decryptedBytes))

		newCfg := &Config{}
		logger.Debug("Reload func: Unmarshalling new application configuration")
		if err := toml.Unmarshal(decryptedBytes, newCfg); err != nil {
			logger.Error("Reload func: Failed to unmarshal new application configuration", "error", err)
			return fmt.Errorf("failed to unmarshal new application configuration: %w", err)
		}
		logger.Debug("Reload func: Successfully unmarshalled new application configuration")

		logger.Debug("Reload func: Validating new application configuration")
		if err := Validate(newCfg); err != nil {
			logger.Error("Reload func: New application configuration validation failed", "error", err)
			return fmt.Errorf("new application configuration validation failed: %w", err)
		}
		logger.Debug("Reload func: New application configuration validated successfully")

		newCfg.Source = "" // Clear source field, as it's now loaded from DB via secure store

		oldCfg := provider.Get()
		restartFields := checkChangedRestartFields(oldCfg, newCfg)

		provider.Update(newCfg)

		if len(restartFields) > 0 {
			logger.Warn("Reload func: Configuration reloaded, but restart required to apply changes", "fields", strings.Join(restartFields, ", "))
		} else {
			logger.Info("Reload func: Application configuration successfully reloaded and updated in provider")
		}

		return nil
	}
}
