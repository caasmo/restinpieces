package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/caasmo/restinpieces/db/zombiezen"
)

var (
	ErrUnknownJobSubcommand = errors.New("unknown job subcommand")
)

func printJobUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, "Usage: %s job <subcommand> [options]\n\n", prog)
	_, _ = fmt.Fprintf(w, "Manages background jobs.\n\n")
	_, _ = fmt.Fprintf(w, "Subcommands:\n")
	_, _ = fmt.Fprintf(w, "  add-backup [options]    Add a new recurrent backup job\n")
	_, _ = fmt.Fprintf(w, "  list [limit]            List jobs in the queue\n")
	_, _ = fmt.Fprintf(w, "  rm <job_id>             Remove a job from the queue\n")
	_, _ = fmt.Fprintf(w, "  add [options]           Add a generic job (advanced)\n")
}

// handleJobCommand is the dispatcher for all "job" subcommands.
func handleJobCommand(dbConn *zombiezen.Db, args []string, ui UI) error {
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
	case "add-backup":
		opts, err := parseJobAddBackupArgs(subcommandArgs)
		if err != nil {
			printJobUsage(ui.Err)
			return err
		}
		return handleJobAddBackupCommand(dbConn, opts, ui)
	case "list":
		return handleJobListCommand(dbConn, subcommandArgs, ui)
	case "rm":
		return handleJobRmCommand(dbConn, subcommandArgs, ui)
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
	case "add-backup":
		return subcommand, subcommandArgs, nil
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
