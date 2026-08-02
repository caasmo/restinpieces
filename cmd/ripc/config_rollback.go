package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/caasmo/restinpieces/config"
)

var (
	// ErrInvalidGeneration is returned when a rollback to a generation less than 1 is attempted.
	ErrInvalidGeneration = errors.New("invalid generation")
)

// handleConfigRollbackCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleConfigRollbackCommand(secureStore config.SecureStore, opts ConfigRollbackOptions, ui UI) error {
	return rollbackConfig(ui.Out, secureStore, opts.Scope, opts.Generation)
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

// ConfigRollbackOptions holds the parsed options for the 'config rollback' subcommand.
type ConfigRollbackOptions struct {
	Scope      string // --scope
	Generation int    // positional generation argument
}

// parseConfigRollbackArgs parses the arguments for the 'rollback' subcommand.
func parseConfigRollbackArgs(args []string) (ConfigRollbackOptions, error) {
	rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
	rollbackCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]

	var opts ConfigRollbackOptions
	rollbackCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := rollbackCmd.Parse(args)
	if err != nil {
		return ConfigRollbackOptions{}, fmt.Errorf("parsing rollback flags: %w: %v", ErrInvalidFlag, err)
	}
	if rollbackCmd.NArg() < 1 {
		return ConfigRollbackOptions{}, fmt.Errorf("'rollback' requires generation number argument: %w", ErrMissingArgument)
	}
	if rollbackCmd.NArg() > 1 {
		return ConfigRollbackOptions{}, fmt.Errorf("'rollback' command takes at most one generation argument: %w", ErrTooManyArguments)
	}
	gen, err := strconv.Atoi(rollbackCmd.Arg(0))
	if err != nil {
		return ConfigRollbackOptions{}, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
	}
	opts.Generation = gen
	return opts, nil
}
