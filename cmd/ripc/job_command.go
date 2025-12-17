package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

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

	subcommand, subcommandArgs, err := parseJobSubcommand(args, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printJobUsage()
		os.Exit(1)
	}

	switch subcommand {
	case "add-backup":
		handleJobAddBackupCommand(dbConn, subcommandArgs[0], subcommandArgs[1], subcommandArgs[2])
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

func parseJobSubcommand(commandArgs []string, output io.Writer) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "add-backup":
		addBackupCmd := flag.NewFlagSet("add-backup", flag.ContinueOnError)
		addBackupCmd.SetOutput(output)
		interval := addBackupCmd.String("interval", "24h", "Interval for the recurrent backup job (e.g., '24h', '1h30m')")
		scheduledFor := addBackupCmd.String("scheduled-for", time.Now().Format(time.RFC3339), "Start time in RFC3339 format for the first job")
		maxAttempts := addBackupCmd.Int("max-attempts", 3, "Maximum number of attempts for the job")
		if err := addBackupCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing add-backup flags: %w: %v", ErrInvalidFlag, err)
		}
		if *interval == "" {
			return "", nil, fmt.Errorf("-interval is a required flag for 'job add-backup': %w", ErrMissingArgument)
		}
		if _, err := time.ParseDuration(*interval); err != nil {
			return "", nil, fmt.Errorf("invalid -interval format: %w", err)
		}
		if _, err := time.Parse(time.RFC3339, *scheduledFor); err != nil {
			return "", nil, fmt.Errorf("invalid -scheduled-for format: %w", err)
		}
		return subcommand, []string{*interval, *scheduledFor, strconv.Itoa(*maxAttempts)}, nil
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

