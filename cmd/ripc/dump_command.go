package main

import (
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// handleDumpCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleDumpCommand(secureStore config.SecureStore, scope string, raw bool) {
	if err := dumpConfig(os.Stdout, secureStore, scope, raw); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// dumpConfig contains the testable core logic for dumping configuration.
// It accepts io.Writer for output, making it easy to test.
func dumpConfig(stdout io.Writer, secureStore config.SecureStore, scope string, raw bool) error {
	if scope == "" {
		scope = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scope, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	if raw {
		_, err = stdout.Write(decryptedData)
		if err != nil {
			return fmt.Errorf("%w: failed to write raw config to stdout: %w", ErrWriteOutput, err)
		}
		return nil
	}

	// Effective dump: Start with defaults
	effective := config.NewDefaultConfig()

	// Merge with stored overrides
	if len(decryptedData) > 0 {
		if err := toml.Unmarshal(decryptedData, effective); err != nil {
			return fmt.Errorf("failed to parse stored config: %w", err)
		}
	}

	// Re-serialize the merged result
	merged, err := toml.Marshal(effective)
	if err != nil {
		return fmt.Errorf("failed to serialize effective config: %w", err)
	}

	_, err = stdout.Write(merged)
	if err != nil {
		return fmt.Errorf("%w: failed to write effective config to stdout: %w", ErrWriteOutput, err)
	}
	return nil
}
