package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/pelletier/go-toml"
)

func handleRenewJwtSecretsCommand(secureStore config.SecureStore) {
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

	// Generate new random secrets
	cfg.Jwt.AuthSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	cfg.Jwt.VerificationEmailSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	cfg.Jwt.PasswordResetSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	cfg.Jwt.EmailChangeSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)

	// Marshal back to TOML
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal config to TOML: %v\n", err)
		os.Exit(1)
	}

	// Save updated config
	err = secureStore.Save(scopeName, tomlBytes, format, "Renewed all JWT secrets")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save renewed JWT secrets for scope '%s': %v\n", scopeName, err)
		os.Exit(1)
	}

	fmt.Println("Successfully renewed all JWT secrets for application scope")
}
