package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"zombiezen.com/go/sqlite/sqlitex"
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
func handleScopesCommand(pool *sqlitex.Pool, args []string, ui UI) error {
	if err := parseScopesArgs(args); err != nil {
		printScopesUsage(ui.Err)
		return err
	}
	return listScopes(ui, pool)
}

// listScopes contains the testable core logic for listing all configuration scopes.
// It accepts UI for output, making it easy to test.
func listScopes(ui UI, pool *sqlitex.Pool) (err error) {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: for scopes command: %w", ErrDbConnection, err)
	}
	defer pool.Put(conn)

	stmt, err := conn.Prepare("SELECT DISTINCT scope FROM app_config ORDER BY scope;")
	if err != nil {
		return fmt.Errorf("%w: for scopes command: %w", ErrDbPrepare, err)
	}
	defer func() {
		// If the function is already returning an error, don't overwrite it
		// with a finalize error. The primary error is usually more important.
		if ferr := stmt.Finalize(); ferr != nil && err == nil {
			err = fmt.Errorf("%w: %w", ErrDbFinalize, ferr)
		}
	}()

	for {
		hasRow, stepErr := stmt.Step()
		if stepErr != nil {
			return fmt.Errorf("%w: %w", ErrDbStep, stepErr)
		}
		if !hasRow {
			break
		}
		scope := stmt.GetText("scope")
		if _, writeErr := fmt.Fprintln(ui.Out, scope); writeErr != nil {
			// Follows the pattern in add_oauth2_command.go for output errors
			return fmt.Errorf("failed to write output: %w", writeErr)
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
