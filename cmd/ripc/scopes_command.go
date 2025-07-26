package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"zombiezen.com/go/sqlite/sqlitex"
)

// Custom errors for the scopes command
var (
	ErrDbConnection = errors.New("failed to get db connection")
	ErrDbPrepare    = errors.New("failed to prepare statement")
	ErrDbStep       = errors.New("failed to step through results")
	ErrDbFinalize   = errors.New("failed to finalize statement")
)

// handleScopesCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleScopesCommand(pool *sqlitex.Pool) {
	if err := listScopes(os.Stdout, pool); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// listScopes contains the testable core logic for listing all configuration scopes.
// It accepts io.Writer for output, making it easy to test.
func listScopes(stdout io.Writer, pool *sqlitex.Pool) (err error) {
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
		if _, writeErr := fmt.Fprintln(stdout, scope); writeErr != nil {
			// Follows the pattern in add_oauth2_command.go for output errors
			return fmt.Errorf("failed to write output: %w", writeErr)
		}
	}

	return nil
}