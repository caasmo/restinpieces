package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
)

// handleJobCommand is the dispatcher for all "job" subcommands.
func handleJobCommand(dbConn *zombiezen.Db, args []string) {
	jobCmd := flag.NewFlagSet("job", flag.ExitOnError)
	jobCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s job <subcommand> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Manages background jobs.\n\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  add-backup [options]    Add a new recurrent backup job\n")
		fmt.Fprintf(os.Stderr, "  list [limit]            List jobs in the queue\n")
		fmt.Fprintf(os.Stderr, "  rm <job_id>             Remove a job from the queue\n")
		fmt.Fprintf(os.Stderr, "  add [options]           Add a generic job (advanced)\n")
	}

	if len(args) < 1 {
		jobCmd.Usage()
		os.Exit(1)
	}

	subcommand := args[0]
	subcommandArgs := args[1:] // The rest of the args for the subcommand

	switch subcommand {
	case "add":
		handleJobAdd(dbConn, subcommandArgs)
	case "add-backup":
		handleJobAddBackup(dbConn, subcommandArgs)
	case "list":
		handleJobList(dbConn, subcommandArgs)
	case "rm":
		handleJobRm(dbConn, subcommandArgs)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown job subcommand: %s\n", subcommand)
		jobCmd.Usage()
		os.Exit(1)
	}
}

// handleJobAdd handles the "job add" subcommand and its specific flags.
func handleJobAdd(dbConn db.DbQueue, args []string) {
	// Create a FlagSet specific to the "add" subcommand.
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)

	// Define flags for the 'add' subcommand
	jobType := addCmd.String("type", "", "Job type (e.g., 'job_type_backup_local') (required)")
	interval := addCmd.String("interval", "", "Interval for recurrent jobs (e.g., '24h')")
	scheduledFor := addCmd.String("scheduled-for", time.Now().Format(time.RFC3339), "Start time in RFC3339 format")
	payloadStr := addCmd.String("payload", "{}", "JSON payload for the job")
	payloadExtraStr := addCmd.String("payload-extra", "{}", "Extra JSON payload for the job")
	recurrent := addCmd.Bool("recurrent", false, "Set if the job is recurrent")
	maxAttempts := addCmd.Int("max-attempts", 3, "Maximum number of attempts")

	addCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ripconf job add [options]\n\n")
		fmt.Fprintf(os.Stderr, "Adds a new job to the queue.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		addCmd.PrintDefaults()
	}

	// Parse the arguments passed specifically to this handler.
	addCmd.Parse(args)

	if *jobType == "" {
		fmt.Fprintln(os.Stderr, "Error: -type is a required flag for 'job add'")
		addCmd.Usage()
		os.Exit(1)
	}

	// Validate payload JSON
	if !json.Valid([]byte(*payloadStr)) {
		fmt.Fprintln(os.Stderr, "Error: -payload is not valid JSON")
		os.Exit(1)
	}
	if !json.Valid([]byte(*payloadExtraStr)) {
		fmt.Fprintln(os.Stderr, "Error: -payload-extra is not valid JSON")
		os.Exit(1)
	}

	// Parse time and duration
	scheduledTime, err := time.Parse(time.RFC3339, *scheduledFor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid -scheduled-for format: %v\n", err)
		os.Exit(1)
	}

	var intervalDuration time.Duration
	if *recurrent {
		if *interval == "" {
			fmt.Fprintln(os.Stderr, "Error: -interval is required for recurrent jobs")
			addCmd.Usage()
			os.Exit(1)
		}
		intervalDuration, err = time.ParseDuration(*interval)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid -interval format: %v\n", err)
			os.Exit(1)
		}
	}

	// Construct the job
	newJob := db.Job{
		JobType:      *jobType,
		Payload:      []byte(*payloadStr),
		PayloadExtra: []byte(*payloadExtraStr),
		ScheduledFor: scheduledTime,
		Recurrent:    *recurrent,
		Interval:     intervalDuration,
		MaxAttempts:  *maxAttempts,
	}

	// Insert the job using the existing DB interface
	if err := dbConn.InsertJob(newJob); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to insert job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully inserted job of type '%s'.\n", newJob.JobType)
}