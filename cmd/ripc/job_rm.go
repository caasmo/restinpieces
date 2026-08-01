package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/db"
)

var (
	ErrDeleteJobFailed = errors.New("failed to delete job")
)

// handleJobRmCommand is the command-level wrapper. It handles parsing command-line
// arguments and calls the core logic.
func handleJobRmCommand(dbConn db.DbQueueAdmin, args []string, ui UI) {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(ui.Err, "Error: 'rm' command requires a job ID")
		os.Exit(1)
	}

	jobID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		_, _ = fmt.Fprintf(ui.Err, "Error: invalid job ID '%s'. Please provide a number.\n", args[0])
		os.Exit(1)
	}

	if err := removeJob(ui.Out, dbConn, jobID); err != nil {
		fprintErr(ui.Err, err)
		os.Exit(1)
	}
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