package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

var ErrUnknownJobSubcommand = errors.New("unknown job subcommand")

func printJobUsage(w io.Writer) {
	help := Spec{
		Usage:       "job <subcommand> [options]",
		Description: "Manages background jobs.",
		Subcommands: []SubcommandGroup{
			{
				Subcommands: []Subcommand{
					{"list [limit]", "List jobs in the queue"},
					{"rm <job_id>", "Remove a job from the queue"},
				},
			},
		},
	}
	help.Print(w, prog, "job")
}

// handleJobCommand is the dispatcher for all "job" subcommands.
func handleJobCommand(db *appDb, args []string, ui UI) error {
	if len(args) < 1 {
		printJobUsage(ui.Err)
		return fmt.Errorf("job requires a subcommand")
	}

	subcommand, subcommandArgs, err := parseJobSubcommand(args)
	if err != nil {
		printJobUsage(ui.Err)
		return err
	}

	switch subcommand {
	case "list":
		return handleJobListCommand(db, subcommandArgs, ui)
	case "rm":
		return handleJobRmCommand(db, subcommandArgs, ui)
	default:
		// This case should ideally not be reached if parseJobSubcommand is correct
		printJobUsage(ui.Err)
		return fmt.Errorf("unknown job subcommand: %s", subcommand)
	}
}

// parseJobSubcommand validates the subcommand name and its positional arguments,
// returning the subcommand and the remaining arguments.
func parseJobSubcommand(commandArgs []string) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "list":
		if len(subcommandArgs) > 1 {
			return "", nil, fmt.Errorf("'list' command takes at most one limit argument: %w", ErrTooManyArguments)
		}
		if len(subcommandArgs) == 1 {
			_, err := strconv.Atoi(subcommandArgs[0])
			if err != nil {
				return "", nil, fmt.Errorf("limit must be a number: %w", ErrNotANumber)
			}
		}
		return subcommand, subcommandArgs, nil
	case "rm":
		if len(subcommandArgs) < 1 {
			return "", nil, fmt.Errorf("'rm' requires job_id argument: %w", ErrMissingArgument)
		}
		if len(subcommandArgs) > 1 {
			return "", nil, fmt.Errorf("'rm' command takes at most one job_id argument: %w", ErrTooManyArguments)
		}
		_, err := strconv.Atoi(subcommandArgs[0])
		if err != nil {
			return "", nil, fmt.Errorf("job_id must be a number: %w", ErrNotANumber)
		}
		return subcommand, subcommandArgs, nil
	default:
		return "", nil, fmt.Errorf("'%s': %w", subcommand, ErrUnknownJobSubcommand)
	}
}
