package main

import (
	"context"
	"fmt"
	"io"

	"zombiezen.com/go/sqlite/sqlitex"
)

// listItems retrieves and prints a formatted list of configurations from the
// database, optionally filtered by scope. It is a testable function that
// prepares and executes a SQL query, then formats the results into a table for display.
func listItems(ui UI, pool *sqlitex.Pool, scopeFilter string) (count int, err error) {
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

	if _, err := fmt.Fprintln(ui.Out, "Gen  Scope        Created At             Format  Description"); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	if _, err := fmt.Fprintln(ui.Out, "---  ------------ ---------------------  ------  -----------"); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

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
		if _, err := fmt.Fprintf(ui.Out, "%3d  %-12s  %-21s  %-4s  %s\n", count, scope, createdAt, format, description); err != nil {
			return count, fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
		count++
	}
	return count, nil

}

func printListUsage(w io.Writer) {
	help := Spec{
		Usage:       "list [scope]",
		Description: "Lists configuration versions.",
		Args: []ArgSpec{
			{"scope", "Optional scope to filter versions by"},
		},
		Examples: []string{
			"ripc list",
			"ripc list my-app",
		},
	}
	help.Print(w, prog)
}

// handleListCommand parses the arguments for the 'list' command and executes
// the core logic, returning any error to the caller.
func handleListCommand(pool *sqlitex.Pool, args []string, ui UI) error {
	opts, err := parseListArgs(args)
	if err != nil {
		printListUsage(ui.Err)
		return err
	}

	count, err := listItems(ui, pool, opts.Scope)
	if err != nil {
		return err
	}

	if count == 0 {
		if opts.Scope != "" {
			_, err := fmt.Fprintf(ui.Err, "No configurations found for scope: %s\n", opts.Scope)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintln(ui.Err, "No configurations found.")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ListOptions holds the parsed options for the 'list' command.
type ListOptions struct {
	Scope string // optional positional scope filter
}

// parseListArgs parses the arguments for the 'list' command.
func parseListArgs(args []string) (ListOptions, error) {
	if len(args) > 1 {
		return ListOptions{}, fmt.Errorf("'list' command takes at most one scope argument: %w", ErrTooManyArguments)
	}
	var opts ListOptions
	if len(args) > 0 {
		opts.Scope = args[0]
	}
	return opts, nil
}
