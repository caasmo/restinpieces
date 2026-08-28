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
		Description: "Manages the default logger database.",
		Subcommands: []SubcommandGroup{
			{
				Subcommands: []Subcommand{
					{"init [logpath]", "Initialize the log database for the default batch logger"},
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

	subcommand, subcommandArgs, err := parseLogSubcommand(commandArgs)
	if err != nil {
		printLogUsage(ui.Err)
		return err
	}

	switch subcommand {
	case "init":
		logPath := ""
		if len(subcommandArgs) > 0 {
			logPath = subcommandArgs[0]
		}
		return handleLogInitCommand(secureStore, dbPath, logPath, ui)
	default:
		printLogUsage(ui.Err)
		return fmt.Errorf("unknown log subcommand: %s", subcommand)
	}
}

func parseLogSubcommand(commandArgs []string) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "init":
		if len(subcommandArgs) > 1 {
			return "", nil, fmt.Errorf("'init' takes at most one log path argument: %w", ErrTooManyArguments)
		}
		return subcommand, subcommandArgs, nil
	default:
		return "", nil, fmt.Errorf("'%s': %w", subcommand, ErrUnknownLogSubcommand)
	}
}
