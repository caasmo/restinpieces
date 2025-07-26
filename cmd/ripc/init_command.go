package main

import (
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// handleInitCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleInitCommand(secureStore config.SecureStore) {
	if err := initializeConfig(os.Stdout, secureStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initializeConfig contains the testable core logic for initializing a default configuration.
// It accepts io.Writer for output, making it easy to test.
func initializeConfig(stdout io.Writer, secureStore config.SecureStore) error {
	scopeName := config.ScopeApplication

	defaultConfig := config.NewDefaultConfig()
	tomlBytes, err := toml.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal default config to TOML: %w", ErrConfigMarshal, err)
	}

	err = secureStore.Save(scopeName, tomlBytes, "toml", "Initial default configuration")
	if err != nil {
		return fmt.Errorf("%w: failed to save default config for scope '%s': %w", ErrSecureStoreSave, scopeName, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully saved default configuration for scope '%s'\n", scopeName); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}
