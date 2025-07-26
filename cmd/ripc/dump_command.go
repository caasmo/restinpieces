package main

import (
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
)

// handleDumpCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleDumpCommand(secureStore config.SecureStore, scope string) {
	if err := dumpConfig(os.Stdout, secureStore, scope); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// dumpConfig contains the testable core logic for dumping configuration.
// It accepts io.Writer for output, making it easy to test.
func dumpConfig(stdout io.Writer, secureStore config.SecureStore, scope string) error {
	if scope == "" {
		scope = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scope, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	_, err = stdout.Write(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to write config to stdout: %w", ErrWriteOutput, err)
	}
	return nil
}
