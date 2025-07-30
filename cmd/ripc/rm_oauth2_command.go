package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

var (
	ErrProviderNotFound = errors.New("provider not found")
)

// handleRmOAuth2Command is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleRmOAuth2Command(secureStore config.SecureStore, providerName string) {
	if err := removeOAuth2Provider(os.Stdout, secureStore, providerName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// removeOAuth2Provider contains the testable core logic for removing an OAuth2 provider.
// It accepts io.Writer for output, making it easy to test.
func removeOAuth2Provider(stdout io.Writer, secureStore config.SecureStore, providerName string) error {
	// Only works with application scope
	scopeName := config.ScopeApplication

	// Get latest config
	decryptedData, format, err := secureStore.Get(scopeName, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve/decrypt latest config for scope '%s': %w", ErrSecureStoreGet, scopeName, err)
	}

	// Load into config struct
	var cfg config.Config
	if err := toml.Unmarshal(decryptedData, &cfg); err != nil {
		return fmt.Errorf("%w: %w", ErrConfigUnmarshal, err)
	}

	// Check if provider exists
	if _, exists := cfg.OAuth2Providers[providerName]; !exists {
		return fmt.Errorf("OAuth2 provider '%s' does not exist: %w", providerName, ErrProviderNotFound)
	}

	// Delete provider
	delete(cfg.OAuth2Providers, providerName)

	// Marshal back to TOML
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal config to TOML: %w", ErrConfigMarshal, err)
	}

	// Save updated config
	err = secureStore.Save(scopeName, tomlBytes, format, fmt.Sprintf("Removed OAuth2 provider: %s", providerName))
	if err != nil {
		return fmt.Errorf("%w: failed to save config after removing OAuth2 provider for scope '%s': %w", ErrSecureStoreSave, scopeName, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully removed OAuth2 provider '%s'\n", providerName); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
