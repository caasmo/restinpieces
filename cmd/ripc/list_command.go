package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"zombiezen.com/go/sqlite/sqlitex"
)

// listItems retrieves and prints a formatted list of configurations from the
// database, optionally filtered by scope. It is a testable function that
// prepares and executes a SQL query, then formats the results into a table for display.
func listItems(stdout io.Writer, pool *sqlitex.Pool, scopeFilter string) (count int, err error) {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return 0, fmt.Errorf("%w: failed to get db connection for list command", ErrDbConnection)
	}
	defer pool.Put(conn)

	query := "SELECT id, scope, created_at, format, description FROM app_config ORDER BY created_at DESC;"
	if scopeFilter != "" {
		query = "SELECT id, scope, created_at, format, description FROM app_config WHERE scope = ? ORDER BY created_at DESC;"
	}

	stmt, err := conn.Prepare(query)
	if err != nil {
		return 0, fmt.Errorf("%w: failed to prepare statement for list command", ErrQueryPrepare)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil && err == nil {
			err = fmt.Errorf("failed to finalize statement: %w", ferr)
		}
	}()

	if scopeFilter != "" {
		stmt.BindText(1, scopeFilter)
	}

	fmt.Fprintln(stdout, "Gen  Scope        Created At             Format  Description")
	fmt.Fprintln(stdout, "---  ------------ ---------------------  ------  -----------")

	for {
		hasRow, stepErr := stmt.Step()
		if stepErr != nil {
			return count, fmt.Errorf("failed to step through list results: %w", stepErr)
		}
		if !hasRow {
			break
		}

		scope := stmt.GetText("scope")
		createdAt := stmt.GetText("created_at")
		format := stmt.GetText("format")
		description := stmt.GetText("description")

		if len(format) > 4 {
			format = format[:4]
		}
		fmt.Fprintf(stdout, "%3d  %-12s  %-21s  %-4s  %s\n", count, scope, createdAt, format, description)
		count++
	}
	return count, nil
}

// handleListCommand is a wrapper around listItems that handles the command-line
// execution, including printing errors to stderr and exiting the program on failure.
func handleListCommand(pool *sqlitex.Pool, scopeFilter string) {
	count, err := listItems(os.Stdout, pool, scopeFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if count == 0 {
		if scopeFilter != "" {
			fmt.Printf("No configurations found for scope: %s\n", scopeFilter)
		} else {
			fmt.Println("No configurations found.")
		}
	}
}