package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/db/zombiezen"
)

var (
	ErrUnknownJobSubcommand = errors.New("unknown job subcommand")
)

func printJobUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s job <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages background jobs.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  add-backup [options]    Add a new recurrent backup job\n")
	fmt.Fprintf(os.Stderr, "  list [limit]            List jobs in the queue\n")
	fmt.Fprintf(os.Stderr, "  rm <job_id>             Remove a job from the queue\n")
	fmt.Fprintf(os.Stderr, "  add [options]           Add a generic job (advanced)\n")
}

// handleJobCommand is the dispatcher for all "job" subcommands.
func handleJobCommand(dbConn *zombiezen.Db, args []string) {
	if len(args) < 1 {
		printJobUsage()
		os.Exit(1)
	}

	subcommand, subcommandArgs, err := parseJobSubcommand(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printJobUsage()
		os.Exit(1)
	}

	switch subcommand {
	case "add-backup":
		handleJobAddBackup(dbConn, subcommandArgs)
	case "list":
		handleJobList(dbConn, subcommandArgs)
	case "rm":
		handleJobRm(dbConn, subcommandArgs)
	default:
		// This case should ideally not be reached if parseJobSubcommand is correct
		fmt.Fprintf(os.Stderr, "Error: unknown job subcommand: %s\n", subcommand)
		printJobUsage()
		os.Exit(1)
	}
}

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