package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"unicode"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// ErrProviderAlreadyExists is returned when trying to add an OAuth2 provider that already exists.
var (
	ErrProviderAlreadyExists = errors.New("provider already exists")
	ErrConfigUnmarshal      = errors.New("failed to unmarshal config")
)

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// handleOAuth2Command is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleOAuth2Command(secureStore config.SecureStore, providerName string) {
	if err := addOAuth2Provider(os.Stdout, secureStore, providerName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// addOAuth2Provider contains the testable core logic for adding a new OAuth2 provider.
// It accepts io.Writer for output, making it easy to test.
func addOAuth2Provider(stdout io.Writer, secureStore config.SecureStore, providerName string) error {
	// Only works with application scope
	scopeName := config.ScopeApplication

	// Get latest config
	decryptedData, format, err := secureStore.Get(scopeName, 0)
	if err != nil {
		return fmt.Errorf("failed to retrieve/decrypt latest config for scope '%s': %w", scopeName, err)
	}

	// Load into config struct
	var cfg config.Config
	if err := toml.Unmarshal(decryptedData, &cfg); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigUnmarshal, err)
	}

	// Check if provider already exists
	if _, exists := cfg.OAuth2Providers[providerName]; exists {
		return fmt.Errorf("OAuth2 provider '%s' already exists: %w", providerName, ErrProviderAlreadyExists)
	}

	// Initialize map if it's nil
	if cfg.OAuth2Providers == nil {
		cfg.OAuth2Providers = make(map[string]config.OAuth2Provider)
	}

	// Add skeleton provider
	cfg.OAuth2Providers[providerName] = config.OAuth2Provider{
		Name:			providerName,
		DisplayName:		capitalizeFirst(providerName),
		RedirectURL:		"",
		RedirectURLPath:	fmt.Sprintf("/oauth2/%s/callback", providerName),
		AuthURL:		"",
		TokenURL:		"",
		UserInfoURL:		"",
		Scopes:			[]string{},
		PKCE:			true,
		ClientID:		"",
		ClientSecret:		"",
	}

	// Marshal back to TOML
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to TOML: %w", err)
	}

	// Save updated config
	err = secureStore.Save(scopeName, tomlBytes, format, fmt.Sprintf("Added OAuth2 provider: %s", providerName))
	if err != nil {
		return fmt.Errorf("failed to save new OAuth2 provider for scope '%s': %w", scopeName, err)
	}

	fmt.Fprintf(stdout, "Successfully added OAuth2 provider '%s'\n", providerName)
	fmt.Fprintln(stdout, "Please configure the provider's URLs, scopes and credentials")
	return nil
}

