package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/caasmo/restinpieces/db"
)

// handleJobCommand is the dispatcher for all "job" subcommands.
func handleJobCommand(dbConn db.DbQueue, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: 'job' command requires a subcommand (e.g., add, list, rm)")
		// TODO: Print job-specific usage from a helper function
		os.Exit(1)
	}

	subcommand := args[0]
	subcommandArgs := args[1:] // The rest of the args for the subcommand

	switch subcommand {
	case "add":
		handleJobAdd(dbConn, subcommandArgs)
	case "list":
		fmt.Println("job list command not yet implemented")
	case "rm":
		fmt.Println("job rm command not yet implemented")
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown job subcommand: %s\n", subcommand)
		// TODO: Print job-specific usage from a helper function
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