package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrUnknownAppSubcommand = errors.New("unknown app subcommand")
)

func printAppUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, "Usage: %s app <subcommand> [options]\n\n", prog)
	_, _ = fmt.Fprintf(w, "Manages the application lifecycle.\n\n")
	_, _ = fmt.Fprintf(w, "Subcommands:\n")
	_, _ = fmt.Fprintf(w, "  create                Create a new application instance\n")
}

func handleAppCommand(secureStore config.SecureStore, dbPool *sqlitex.Pool, dbPath string, commandArgs []string, ui UI) {
	if len(commandArgs) < 1 {
		printAppUsage(ui.Err)
		os.Exit(1)
	}

	subcommand, _, err := parseAppSubcommand(commandArgs, ui.Err)
	if err != nil {
		fprintErr(ui.Err, err)
		printAppUsage(ui.Err)
		os.Exit(1)
	}

	switch subcommand {
	case "create":
		handleAppCreateCommand(secureStore, dbPool, dbPath, ui)
	default:
		// This case should ideally not be reached if parseAppSubcommand is correct
		_, _ = fmt.Fprintf(ui.Err, "Error: unknown app subcommand: %s\n", subcommand)
		printAppUsage(ui.Err)
		os.Exit(1)
	}
}

func parseAppSubcommand(commandArgs []string, output io.Writer) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "create":
		createCmd := flag.NewFlagSet("create", flag.ContinueOnError)
		createCmd.SetOutput(output)
		if err := createCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing create flags: %w: %v", ErrInvalidFlag, err)
		}
		if createCmd.NArg() > 0 {
			return "", nil, fmt.Errorf("'create' does not take any arguments: %w", ErrTooManyArguments)
		}
		return subcommand, nil, nil
	default:
		return "", nil, fmt.Errorf("'%s': %w", subcommand, ErrUnknownAppSubcommand)
	}
}