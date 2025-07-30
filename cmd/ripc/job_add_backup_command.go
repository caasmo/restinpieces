package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/handlers"
)

var (
	ErrInsertJobFailed = errors.New("failed to insert job")
)

// handleJobAddBackup handles the "job add-backup" subcommand. It's the command-line wrapper.
func handleJobAddBackup(dbConn db.DbQueue, args []string) {
	addBackupCmd := flag.NewFlagSet("add-backup", flag.ContinueOnError)

	interval := addBackupCmd.String("interval", "24h", "Interval for the recurrent backup job (e.g., '24h', '1h30m')")
	scheduledFor := addBackupCmd.String("scheduled-for", time.Now().Format(time.RFC3339), "Start time in RFC3339 format for the first job")
	maxAttempts := addBackupCmd.Int("max-attempts", 3, "Maximum number of attempts for the job")

	addBackupCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ripconf job add-backup [options]\n\n")
		fmt.Fprintf(os.Stderr, "Adds a new recurrent backup job to the queue.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		addBackupCmd.PrintDefaults()
	}

	if err := addBackupCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing add-backup flags: %v\n", err)
		addBackupCmd.Usage()
		os.Exit(1)
	}

	// --- Parse and validate flags ---
	if *interval == "" {
		fmt.Fprintln(os.Stderr, "Error: -interval is a required flag for 'job add-backup'")
		addBackupCmd.Usage()
		os.Exit(1)
	}

	intervalDuration, err := time.ParseDuration(*interval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid -interval format: %v\n", err)
		os.Exit(1)
	}

	scheduledTime, err := time.Parse(time.RFC3339, *scheduledFor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid -scheduled-for format: %v\n", err)
		os.Exit(1)
	}

	if err := addBackupJob(os.Stdout, dbConn, intervalDuration, scheduledTime, *maxAttempts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// addBackupJob contains the testable core logic for adding a backup job.
func addBackupJob(stdout io.Writer, dbConn db.DbQueue, interval time.Duration, scheduledFor time.Time, maxAttempts int) error {
	// --- Construct the job ---
	newJob := db.Job{
		JobType:      handlers.JobTypeBackupLocal,
		Payload:      []byte("{}"), // No payload needed for this job type
		PayloadExtra: []byte("{}"), // No extra payload needed
		ScheduledFor: scheduledFor,
		Recurrent:    true, // This is always a recurrent job
		Interval:     interval,
		MaxAttempts:  maxAttempts,
	}

	// --- Insert the job into the database ---
	if err := dbConn.InsertJob(newJob); err != nil {
		return fmt.Errorf("%w: %v", ErrInsertJobFailed, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully inserted recurrent backup job of type '%s'.\n", newJob.JobType); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	if _, err := fmt.Fprintf(stdout, "  - Interval: %s\n", newJob.Interval); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	if _, err := fmt.Fprintf(stdout, "  - First run scheduled for: %s\n", newJob.ScheduledFor.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}

	return nil
}