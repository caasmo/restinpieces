package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

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

// parseRollbackArgs parses the arguments for the 'rollback' subcommand.
func parseRollbackArgs(args []string) (scope string, generation int, err error) {
	rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
	rollbackCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	rollbackScope := rollbackCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	if err := rollbackCmd.Parse(args); err != nil {
		return "", 0, fmt.Errorf("parsing rollback flags: %w: %v", ErrInvalidFlag, err)
	}
	if rollbackCmd.NArg() < 1 {
		return "", 0, fmt.Errorf("'rollback' requires generation number argument: %w", ErrMissingArgument)
	}
	if rollbackCmd.NArg() > 1 {
		return "", 0, fmt.Errorf("'rollback' command takes at most one generation argument: %w", ErrTooManyArguments)
	}
	gen, err := strconv.Atoi(rollbackCmd.Arg(0))
	if err != nil {
		return "", 0, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
	}
	return *rollbackScope, gen, nil
}
