package main

import (
	"errors"
	"fmt"
	"io"
)

// Custom errors for the scopes command
var (
	ErrDbPrepare  = errors.New("failed to prepare statement")
	ErrDbStep     = errors.New("failed to step through results")
	ErrDbFinalize = errors.New("failed to finalize statement")
)

func printScopesUsage(w io.Writer) {
	help := Spec{
		Usage:       "scopes",
		Description: "Lists all configuration scopes.",
		Examples: []string{
			"ripc scopes",
		},
	}
	help.Print(w, prog)
}

// handleScopesCommand parses the arguments for the 'scopes' command and
// executes the core logic, returning any error to the caller.
func handleScopesCommand(db *appDb, args []string, ui UI) error {
	if err := parseScopesArgs(args); err != nil {
		printScopesUsage(ui.Err)
		return err
	}
	return listScopes(ui, db)
}

// listScopes contains the testable core logic for listing all configuration scopes.
// It accepts UI for output, making it easy to test.
func listScopes(ui UI, db *appDb) error {
	scopes, err := db.configScopes()
	if err != nil {
		return err
	}
	for _, s := range scopes {
		if _, err := fmt.Fprintln(ui.Out, s); err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
	}
	return nil
}

// parseScopesArgs parses the arguments for the 'scopes' command.
func parseScopesArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("'scopes' command does not take any arguments: %w", ErrTooManyArguments)
	}
	return nil
}
