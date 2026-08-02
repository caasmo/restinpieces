package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/caasmo/restinpieces/db"
)

var (
	ErrDeleteJobFailed = errors.New("failed to delete job")
)

// handleJobRmCommand is the command-level wrapper. It handles parsing command-line
// arguments and calls the core logic.
func handleJobRmCommand(dbConn db.DbQueueAdmin, args []string, ui UI) error {
	if len(args) < 1 {
		return fmt.Errorf("'rm' command requires a job ID: %w", ErrMissingArgument)
	}

	jobID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid job ID '%s'. Please provide a number: %w", args[0], ErrNotANumber)
	}

	return removeJob(ui.Out, dbConn, jobID)
}

// removeJob contains the testable core logic for removing a job from the queue.
func removeJob(stdout io.Writer, dbConn db.DbQueueAdmin, jobID int64) error {
	err := dbConn.DeleteJob(jobID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteJobFailed, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully deleted job %d\n", jobID); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	return nil
}
