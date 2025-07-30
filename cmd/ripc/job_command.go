package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/db/zombiezen"
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

	subcommand := args[0]
	subcommandArgs := args[1:] // The rest of the args for the subcommand

	switch subcommand {
	case "add-backup":
		handleJobAddBackup(dbConn, subcommandArgs)
	case "list":
		handleJobList(dbConn, subcommandArgs)
	case "rm":
		handleJobRm(dbConn, subcommandArgs)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown job subcommand: %s\n", subcommand)
		printJobUsage()
		os.Exit(1)
	}
}

