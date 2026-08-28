package main

import (
	"errors"
	"fmt"
	"strconv"

	rdb "github.com/caasmo/restinpieces/db"
)

var (
	ErrDeleteJobFailed = errors.New("failed to delete job")
)

// handleJobRmCommand is the command-level wrapper. It handles parsing command-line
// arguments and calls the core logic.
func handleJobRmCommand(dbConn rdb.DbQueueAdmin, args []string, ui UI) error {
	if len(args) < 1 {
		return fmt.Errorf("'rm' command requires a job ID: %w", ErrMissingArgument)
	}

	jobID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid job ID '%s'. Please provide a number: %w", args[0], ErrNotANumber)
	}

	return removeJob(ui, dbConn, jobID)
}

// removeJob contains the testable core logic for removing a job from the queue.
func removeJob(ui UI, dbConn rdb.DbQueueAdmin, jobID int64) error {
	err := dbConn.DeleteJob(jobID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteJobFailed, err)
	}

	if _, err := fmt.Fprintf(ui.Err, "Successfully deleted job %d\n", jobID); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	return nil
}
