package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/caasmo/restinpieces/db"
)

var (
	ErrListJobsFailed = errors.New("failed to list jobs")
)

func handleJobList(dbConn db.DbQueueAdmin, args []string) {
	limit := 0 // Default to all jobs
	if len(args) > 0 {
		var err error
		limit, err = strconv.Atoi(args[0])
		if err != nil {
			// This error is from argument parsing, not the core logic, so it can stay simple.
			fmt.Fprintf(os.Stderr, "Error: invalid limit '%s'. Please provide a number.\n", args[0])
			os.Exit(1)
		}
	}

	if err := listJobs(os.Stdout, dbConn, limit); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func listJobs(stdout io.Writer, dbConn db.DbQueueAdmin, limit int) error {
	jobs, err := dbConn.ListJobs(limit)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrListJobsFailed, err)
	}

	if len(jobs) == 0 {
		if _, err := fmt.Fprintln(stdout, "No jobs found in the queue."); err != nil {
			return fmt.Errorf("%w: %v", ErrWriteOutput, err)
		}
		return nil
	}

	// Format the output using a tabwriter for alignment
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tSCHEDULED FOR\tINTERVAL\tATTEMPTS\tPAYLOAD\tPAYLOAD EXTRA\tLAST ERROR"); err != nil {
		return fmt.Errorf("%w: failed to write header: %v", ErrWriteOutput, err)
	}
	if _, err := fmt.Fprintln(w, "--\t----\t------\t-------------\t--------\t--------\t-------\t-------------\t----------"); err != nil {
		return fmt.Errorf("%w: failed to write header separator: %v", ErrWriteOutput, err)
	}

	for _, job := range jobs {
		scheduledFor := "N/A"
		if !job.ScheduledFor.IsZero() {
			scheduledFor = job.ScheduledFor.Format(time.RFC3339)
		}

		interval := "N/A"
		if job.Recurrent {
			interval = job.Interval.String()
		}

		payload := string(job.Payload)
		if len(payload) > 20 {
			payload = payload[:17] + "..."
		}

		payloadExtra := string(job.PayloadExtra)
		if len(payloadExtra) > 20 {
			payloadExtra = payloadExtra[:17] + "..."
		}

		lastError := job.LastError
		if len(lastError) > 50 {
			lastError = lastError[:47] + "..."
		}

		if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d/%d\t%s\t%s\t%s\n",
			job.ID,
			job.JobType,
			job.Status,
			scheduledFor,
			interval,
			job.Attempts,
			job.MaxAttempts,
			payload,
			payloadExtra,
			lastError,
		); err != nil {
			return fmt.Errorf("%w: failed to write job list item: %v", ErrWriteOutput, err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("%w: failed to flush output: %v", ErrWriteOutput, err)
	}
	return nil
}