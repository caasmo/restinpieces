package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

func handleRmOAuth2Command(secureStore config.SecureStore, providerName string) {
	// Only works with application scope
	scopeName := config.ScopeApplication

	// Get latest config
	decryptedData, format, err := secureStore.Get(scopeName, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to retrieve/decrypt latest config for scope '%s': %v\n", scopeName, err)
		os.Exit(1)
	}

	// Load into config struct
	var cfg config.Config
	if err := toml.Unmarshal(decryptedData, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to unmarshal config TOML: %v\n", err)
		os.Exit(1)
	}

	// Check if provider exists
	if _, exists := cfg.OAuth2Providers[providerName]; !exists {
		fmt.Fprintf(os.Stderr, "Error: OAuth2 provider '%s' does not exist\n", providerName)
		os.Exit(1)
	}

	// Delete provider
	delete(cfg.OAuth2Providers, providerName)

	// Marshal back to TOML
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal config to TOML: %v\n", err)
		os.Exit(1)
	}

	// Save updated config
	err = secureStore.Save(scopeName, tomlBytes, format, fmt.Sprintf("Removed OAuth2 provider: %s", providerName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save config after removing OAuth2 provider for scope '%s': %v\n", scopeName, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully removed OAuth2 provider '%s'\n", providerName)
}
