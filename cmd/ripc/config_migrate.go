package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// handleMigrateCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleMigrateCommand(secureStore config.SecureStore) {
	err := migrateConfig(os.Stdout, secureStore)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// migrateConfig contains the testable core logic for migrating configuration.
// If no existing config is found, it creates a fresh default config.
// If an existing config is found, it merges stored values onto framework defaults,
// stripping stale keys and filling in new defaults.
func migrateConfig(stdout io.Writer, secureStore config.SecureStore) error {
	scopeName := config.ScopeApplication

	decryptedData, _, err := secureStore.Get(scopeName, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve config for scope '%s': %w", ErrSecureStoreGet, scopeName, err)
	}

	if len(decryptedData) == 0 {
		// No existing config — fresh init path
		defaultConfig := config.NewDefaultConfig()
		tomlBytes, marshalErr := toml.Marshal(defaultConfig)
		if marshalErr != nil {
			return fmt.Errorf("%w: failed to marshal default config to TOML: %w", ErrConfigMarshal, marshalErr)
		}
		saveErr := secureStore.Save(scopeName, tomlBytes, "toml", "Initial default configuration via migrate")
		if saveErr != nil {
			return fmt.Errorf("%w: failed to save default config for scope '%s': %w", ErrSecureStoreSave, scopeName, saveErr)
		}
		_, err = fmt.Fprintf(stdout, "No existing config found. Saved default configuration for scope '%s'\n", scopeName)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
		return nil
	}

	// Existing config found — merge path
	merged := config.NewDefaultConfig()
	unmarshalErr := toml.Unmarshal(decryptedData, merged)
	if unmarshalErr != nil {
		return fmt.Errorf("%w: failed to parse stored config for scope '%s': %w", ErrConfigUnmarshal, scopeName, unmarshalErr)
	}

	tomlBytes, marshalErr := toml.Marshal(merged)
	if marshalErr != nil {
		return fmt.Errorf("%w: failed to marshal migrated config: %w", ErrConfigMarshal, marshalErr)
	}

	saveErr := secureStore.Save(scopeName, tomlBytes, "toml", "Config migrated to current framework version")
	if saveErr != nil {
		return fmt.Errorf("%w: failed to save migrated config for scope '%s': %w", ErrSecureStoreSave, scopeName, saveErr)
	}

	_, err = fmt.Fprintf(stdout, "Config migrated successfully for scope '%s'. Stale keys removed, new defaults applied.\n", scopeName)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

// parseMigrateArgs parses the arguments for the 'migrate' subcommand.
func parseMigrateArgs(args []string) error {
	migrateCmd := flag.NewFlagSet("migrate", flag.ContinueOnError)
	migrateCmd.SetOutput(io.Discard)
	err := migrateCmd.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing migrate flags: %w: %v", ErrInvalidFlag, err)
	}
	if migrateCmd.NArg() > 0 {
		return fmt.Errorf("'migrate' does not take any arguments: %w", ErrTooManyArguments)
	}
	return nil
}
