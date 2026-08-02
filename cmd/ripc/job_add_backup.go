package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/handlers"
)

var (
	ErrInsertJobFailed = errors.New("failed to insert job")
)

// handleJobAddBackupCommand handles the "job add-backup" subcommand. It's the command-line wrapper.
func handleJobAddBackupCommand(dbConn db.DbQueue, interval, scheduledFor, maxAttemptsStr string, ui UI) error {
	// --- Parse and validate flags ---
	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid -interval format: %w", err)
	}

	scheduledTime, err := time.Parse(time.RFC3339, scheduledFor)
	if err != nil {
		return fmt.Errorf("invalid -scheduled-for format: %w", err)
	}

	maxAttempts, err := strconv.Atoi(maxAttemptsStr)
	if err != nil {
		return fmt.Errorf("invalid -max-attempts format: %w", err)
	}

	return addBackupJob(ui.Out, dbConn, intervalDuration, scheduledTime, maxAttempts)
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
