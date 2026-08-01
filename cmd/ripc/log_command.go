package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
)

var (
	ErrUnknownLogSubcommand = errors.New("unknown log subcommand")
)

func printLogUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, "Usage: %s log <subcommand> [options]\n\n", prog)
	_, _ = fmt.Fprintf(w, "Manages the logger database.\n\n")
	_, _ = fmt.Fprintf(w, "Subcommands:\n")
	_, _ = fmt.Fprintf(w, "  init    Initialize the log database and schema\n")
}

func handleLogCommand(secureStore config.SecureStore, dbPath string, commandArgs []string, ui UI) {
	if len(commandArgs) < 1 {
		printLogUsage(ui.Err)
		os.Exit(1)
	}

	subcommand, _, err := parseLogSubcommand(commandArgs)
	if err != nil {
		fprintErr(ui.Err, err)
		printLogUsage(ui.Err)
		os.Exit(1)
	}

	switch subcommand {
	case "init":
		handleLogInitCommand(secureStore, dbPath, ui)
	default:
		// This case should ideally not be reached if parseLogSubcommand is correct
		_, _ = fmt.Fprintf(ui.Err, "Error: unknown log subcommand: %s\n", subcommand)
		printLogUsage(ui.Err)
		os.Exit(1)
	}
}

func parseLogSubcommand(commandArgs []string) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "init":
		if len(subcommandArgs) > 0 {
			return "", nil, fmt.Errorf("'init' does not take any arguments: %w", ErrTooManyArguments)
		}
		return subcommand, nil, nil
	default:
		return "", nil, fmt.Errorf("'%s': %w", subcommand, ErrUnknownLogSubcommand)
	}
}