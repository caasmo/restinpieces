package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
)

var (
	ErrUnknownLogSubcommand = errors.New("unknown log subcommand")
)

func printLogUsage(w io.Writer) {
	help := Spec{
		Usage:       "log <subcommand> [options]",
		Description: "Manages the logger database.",
		Subcommands: []SubcommandGroup{
			{
				Subcommands: []Subcommand{
					{"init", "Initialize the log database and schema"},
				},
			},
		},
	}
	help.Print(w, prog, "log")
}

func handleLogCommand(secureStore config.SecureStore, dbPath string, commandArgs []string, ui UI) error {
	if len(commandArgs) < 1 {
		printLogUsage(ui.Err)
		return fmt.Errorf("log requires a subcommand")
	}

	subcommand, _, err := parseLogSubcommand(commandArgs)
	if err != nil {
		printLogUsage(ui.Err)
		return err
	}

	switch subcommand {
	case "init":
		return handleLogInitCommand(secureStore, dbPath, ui)
	default:
		// This case should ideally not be reached if parseLogSubcommand is correct
		printLogUsage(ui.Err)
		return fmt.Errorf("unknown log subcommand: %s", subcommand)
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
