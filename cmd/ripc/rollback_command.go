package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
)

var (
	// ErrInvalidGeneration is returned when a rollback to a generation less than 1 is attempted.
	ErrInvalidGeneration = errors.New("invalid generation")
)

// handleRollbackCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleRollbackCommand(secureStore config.SecureStore, scope string, generation int) {
	if err := rollbackConfig(os.Stdout, secureStore, scope, generation); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// rollbackConfig contains the testable core logic for rolling back a configuration.
// It accepts io.Writer for output, making it easy to test.
func rollbackConfig(stdout io.Writer, secureStore config.SecureStore, scope string, generation int) error {
	if scope == "" {
		scope = config.ScopeApplication
	}

	if generation < 1 {
		return fmt.Errorf("can only rollback to generation 1 or higher: %w", ErrInvalidGeneration)
	}

	// Get the target generation config
	targetData, format, err := secureStore.Get(scope, generation)
	if err != nil {
		return fmt.Errorf("%w: failed to get config generation %d for scope '%s': %w", ErrSecureStoreGet, generation, scope, err)
	}

	// Save it as the new latest version
	description := fmt.Sprintf("Rollback to generation %d", generation)
	err = secureStore.Save(scope, targetData, format, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save rollback config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully rolled back scope '%s' to generation %d\n", scope, generation); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}
