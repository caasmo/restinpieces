package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
)

var (
	ErrUnknownAppSubcommand = errors.New("unknown app subcommand")
)

func printAppUsage(w io.Writer) {
	help := Spec{
		Usage:       "app <subcommand> [options]",
		Description: "Manages the application lifecycle.",
		Subcommands: []SubcommandGroup{
			{
				Subcommands: []Subcommand{
					{"create", "Create a new application instance"},
				},
			},
		},
	}
	help.Print(w, prog, "app")
}

func handleAppCommand(secureStore config.SecureStore, ripcDb *db, dbPath string, commandArgs []string, ui UI) error {
	if len(commandArgs) < 1 {
		printAppUsage(ui.Err)
		return fmt.Errorf("app requires a subcommand")
	}

	subcommand, _, err := parseAppSubcommand(commandArgs)
	if err != nil {
		printAppUsage(ui.Err)
		return err
	}

	switch subcommand {
	case "create":
		return handleAppCreateCommand(secureStore, ripcDb, dbPath, ui)
	default:
		// This case should ideally not be reached if parseAppSubcommand is correct
		printAppUsage(ui.Err)
		return fmt.Errorf("unknown app subcommand: %s", subcommand)
	}
}

func parseAppSubcommand(commandArgs []string) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "create":
		createCmd := flag.NewFlagSet("create", flag.ContinueOnError)
		createCmd.SetOutput(io.Discard)
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
