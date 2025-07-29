package main

import (
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/pelletier/go-toml"
)

// handleRotateJwtSecretsCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleRotateJwtSecretsCommand(secureStore config.SecureStore) {
	if err := rotateJwtSecrets(os.Stdout, secureStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// rotateJwtSecrets contains the testable core logic for rotating all JWT secrets.
// It accepts io.Writer for output, making it easy to test.
func rotateJwtSecrets(stdout io.Writer, secureStore config.SecureStore) error {
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

	// Generate new random secrets
	cfg.Jwt.AuthSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	cfg.Jwt.VerificationEmailSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	cfg.Jwt.PasswordResetSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	cfg.Jwt.EmailChangeSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)

	// Marshal back to TOML
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal config to TOML: %w", ErrConfigMarshal, err)
	}

	// Save updated config
	err = secureStore.Save(scopeName, tomlBytes, format, "Renewed all JWT secrets")
	if err != nil {
		return fmt.Errorf("%w: failed to save renewed JWT secrets for scope '%s': %w", ErrSecureStoreSave, scopeName, err)
	}

	if _, err := fmt.Fprintln(stdout, "Successfully renewed all JWT secrets for application scope"); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}
