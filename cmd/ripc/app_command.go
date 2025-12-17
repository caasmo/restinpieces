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

func printAppUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s app <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages the application lifecycle.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  create                Create a new application instance\n")
}

func handleAppCommand(secureStore config.SecureStore, dbPool *sqlitex.Pool, dbPath string, commandArgs []string) {
	if len(commandArgs) < 1 {
		printAppUsage()
		os.Exit(1)
	}

	subcommand, _, err := parseAppSubcommand(os.Stderr, commandArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printAppUsage()
		os.Exit(1)
	}

	switch subcommand {
	case "create":
		handleAppCreateCommand(secureStore, dbPool, dbPath)
	default:
		// This case should ideally not be reached if parseAppSubcommand is correct
		fmt.Fprintf(os.Stderr, "Error: unknown app subcommand: %s\n", subcommand)
		printAppUsage()
		os.Exit(1)
	}
}

func parseAppSubcommand(output io.Writer, commandArgs []string) (string, []string, error) {
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