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

func printRollbackUsage(w io.Writer) {
	help := Spec{
		Usage:       "rollback [options] <generation>",
		Description: "Restores a previous configuration version.",
		Args: []ArgSpec{
			{"generation", "Generation number to restore"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
		},
		Examples: []string{
			"ripc rollback 3",
			"ripc rollback --scope my-app 3",
		},
	}
	help.Print(w, prog)
}

// handleRollbackCommand parses the arguments for the 'rollback' command and
// executes the core logic, returning any error to the caller.
func handleRollbackCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseRollbackArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printRollbackUsage(ui.Out)
			return nil
		}
		printRollbackUsage(ui.Err)
		return err
	}
	return rollbackConfig(ui, secureStore, opts.Scope, opts.Generation)
}

// rollbackConfig contains the testable core logic for rolling back a configuration.
// It accepts UI for output, making it easy to test.
func rollbackConfig(ui UI, secureStore config.SecureStore, scope string, generation int) error {
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

	if _, err := fmt.Fprintf(ui.Err, "Successfully rolled back scope '%s' to generation %d\n", scope, generation); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

// RollbackOptions holds the parsed options for the 'rollback' command.
type RollbackOptions struct {
	Scope      string // --scope
	Generation int    // positional generation argument
}

// parseRollbackArgs parses the arguments for the 'rollback' command.
func parseRollbackArgs(args []string) (RollbackOptions, error) {
	rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
	rollbackCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")

	var opts RollbackOptions
	rollbackCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := rollbackCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return RollbackOptions{}, flag.ErrHelp
		}
		return RollbackOptions{}, fmt.Errorf("parsing rollback flags: %w: %v", ErrInvalidFlag, err)
	}
	if rollbackCmd.NArg() < 1 {
		return RollbackOptions{}, fmt.Errorf("'rollback' requires generation number argument: %w", ErrMissingArgument)
	}
	if rollbackCmd.NArg() > 1 {
		return RollbackOptions{}, fmt.Errorf("'rollback' command takes at most one generation argument: %w", ErrTooManyArguments)
	}
	gen, err := strconv.Atoi(rollbackCmd.Arg(0))
	if err != nil {
		return RollbackOptions{}, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
	}
	opts.Generation = gen
	return opts, nil
}
