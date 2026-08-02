package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/handlers"
)

var (
	ErrInsertJobFailed = errors.New("failed to insert job")
)

// handleJobAddBackupCommand handles the "job add backup" subcommand. It's the command-line wrapper.
func handleJobAddBackupCommand(dbConn db.DbQueue, opts JobAddBackupOptions, ui UI) error {
	intervalDuration, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return fmt.Errorf("invalid -interval format: %w", err)
	}

	scheduledTime, err := time.Parse(time.RFC3339, opts.ScheduledFor)
	if err != nil {
		return fmt.Errorf("invalid -scheduled-for format: %w", err)
	}

	return addBackupJob(ui, dbConn, intervalDuration, scheduledTime, opts.MaxAttempts)
}

// JobAddBackupOptions holds the parsed options for the 'job add backup' subcommand.
type JobAddBackupOptions struct {
	Interval     string // -interval, validated as a time.Duration
	ScheduledFor string // -scheduled-for, validated as RFC3339
	MaxAttempts  int    // -max-attempts
}

// parseJobAddBackupArgs parses the arguments for the 'job add backup' subcommand.
func parseJobAddBackupArgs(args []string) (JobAddBackupOptions, error) {
	addBackupCmd := flag.NewFlagSet("add backup", flag.ContinueOnError)
	addBackupCmd.SetOutput(io.Discard)

	var opts JobAddBackupOptions
	addBackupCmd.StringVar(&opts.Interval, "interval", "1m", "Interval for the recurrent backup job (e.g., '24h', '1h30m')")
	addBackupCmd.StringVar(&opts.ScheduledFor, "scheduled-for", time.Now().Format(time.RFC3339), "Start time in RFC3339 format for the first job")
	addBackupCmd.IntVar(&opts.MaxAttempts, "max-attempts", 3, "Maximum number of attempts for the job")

	err := addBackupCmd.Parse(args)
	if err != nil {
		return JobAddBackupOptions{}, fmt.Errorf("parsing add backup flags: %w: %v", ErrInvalidFlag, err)
	}
	if opts.Interval == "" {
		return JobAddBackupOptions{}, fmt.Errorf("-interval is a required flag for 'job add backup': %w", ErrMissingArgument)
	}
	_, err = time.ParseDuration(opts.Interval)
	if err != nil {
		return JobAddBackupOptions{}, fmt.Errorf("invalid -interval format: %w", err)
	}
	_, err = time.Parse(time.RFC3339, opts.ScheduledFor)
	if err != nil {
		return JobAddBackupOptions{}, fmt.Errorf("invalid -scheduled-for format: %w", err)
	}
	return opts, nil
}

// addBackupJob contains the testable core logic for adding a backup job.
func addBackupJob(ui UI, dbConn db.DbQueue, interval time.Duration, scheduledFor time.Time, maxAttempts int) error {
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

	if _, err := fmt.Fprintf(ui.Err, "Successfully inserted recurrent backup job of type '%s'.\n", newJob.JobType); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	if _, err := fmt.Fprintf(ui.Err, "  - Interval: %s\n", newJob.Interval); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	if _, err := fmt.Fprintf(ui.Err, "  - First run scheduled for: %s\n", newJob.ScheduledFor.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}

	return nil
}
