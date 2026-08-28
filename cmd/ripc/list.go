package main

import (
	"fmt"
	"io"
)

func printConfigList(ui UI, ripcDb *db, scopeFilter string) (count int, err error) {
	rows, err := ripcDb.configList(scopeFilter)
	if err != nil {
		return 0, err
	}

	_, err = fmt.Fprintln(ui.Out, "Gen  Scope        Created At             Format  Description")
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	_, err = fmt.Fprintln(ui.Out, "---  ------------ ---------------------  ------  -----------")
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	for i, r := range rows {
		format := r.Format
		if len(format) > 4 {
			format = format[:4]
		}

		_, err = fmt.Fprintf(ui.Out, "%3d  %-12s  %-21s  %-4s  %s\n", i, r.Scope, r.CreatedAt, format, r.Description)
		if err != nil {
			return i, fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
	}

	return len(rows), nil
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
func handleListCommand(ripcDb *db, args []string, ui UI) error {
	opts, err := parseListArgs(args)
	if err != nil {
		printListUsage(ui.Err)
		return err
	}

	count, err := printConfigList(ui, ripcDb, opts.Scope)
	if err != nil {
		return err
	}

	if count == 0 {
		if opts.Scope != "" {
			_, err = fmt.Fprintf(ui.Err, "No configurations found for scope: %s\n", opts.Scope)
			if err != nil {
				return err
			}
		} else {
			_, err = fmt.Fprintln(ui.Err, "No configurations found.")
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
