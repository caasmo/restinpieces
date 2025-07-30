package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

var (
	ErrUnknownLogSubcommand = errors.New("unknown log subcommand")
)

func printLogUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s log <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages the logger database.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  init    Initialize the log database and schema\n")
}

func handleLogCommand(secureStore config.SecureStore, dbPath string, commandArgs []string) {
	if len(commandArgs) < 1 {
		printLogUsage()
		os.Exit(1)
	}

	subcommand, _, err := parseLogSubcommand(commandArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printLogUsage()
		os.Exit(1)
	}

	switch subcommand {
	case "init":
		handleLogInitCommand(secureStore, dbPath)
	default:
		// This case should ideally not be reached if parseLogSubcommand is correct
		fmt.Fprintf(os.Stderr, "Error: unknown log subcommand: %s\n", subcommand)
		printLogUsage()
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