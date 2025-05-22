package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

func handleOAuth2Command(secureStore config.SecureStore, providerName string) {
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

	// Check if provider already exists
	if _, exists := cfg.OAuth2Providers[providerName]; exists {
		fmt.Fprintf(os.Stderr, "Error: OAuth2 provider '%s' already exists\n", providerName)
		os.Exit(1)
	}

	// Add skeleton provider
	cfg.OAuth2Providers[providerName] = config.OAuth2Provider{
		Name:            providerName,
		DisplayName:     strings.Title(providerName),
		RedirectURL:     "",
		RedirectURLPath: fmt.Sprintf("/oauth2/%s/callback", providerName),
		AuthURL:         "",
		TokenURL:        "",
		UserInfoURL:     "",
		Scopes:          []string{},
		PKCE:            true,
		ClientID:        "",
		ClientSecret:    "",
	}

	// Marshal back to TOML
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal config to TOML: %v\n", err)
		os.Exit(1)
	}

	// Save updated config
	err = secureStore.Save(scopeName, tomlBytes, format, fmt.Sprintf("Added OAuth2 provider: %s", providerName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save new OAuth2 provider for scope '%s': %v\n", scopeName, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully added OAuth2 provider '%s'\n", providerName)
	fmt.Println("Please configure the provider's URLs, scopes and credentials")
}
