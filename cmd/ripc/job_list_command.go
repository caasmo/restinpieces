package main

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/caasmo/restinpieces/db"
)

func handleJobList(dbConn db.DbQueueAdmin, args []string) {
	limit := 0 // Default to all jobs
	if len(args) > 0 {
		var err error
		limit, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid limit '%s'. Please provide a number.\n", args[0])
			os.Exit(1)
		}
	}

	jobs, err := dbConn.ListJobs(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list jobs: %v\n", err)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found in the queue.")
		return
	}

	// Format the output using a tabwriter for alignment
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tSCHEDULED FOR\tINTERVAL\tATTEMPTS\tLAST ERROR")
	fmt.Fprintln(w, "--\t----\t------\t-------------\t--------\t--------\t----------")

	for _, job := range jobs {
		scheduledFor := "N/A"
		if !job.ScheduledFor.IsZero() {
			scheduledFor = job.ScheduledFor.Format(time.RFC3339)
		}

		interval := "N/A"
		if job.Recurrent {
			interval = job.Interval.String()
		}

		lastError := job.LastError
		if len(lastError) > 50 {
			lastError = lastError[:47] + "..."
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d/%d\t%s\n",
			job.ID,
			job.JobType,
			job.Status,
			scheduledFor,
			interval,
			job.Attempts,
			job.MaxAttempts,
			lastError,
		)
	}

	w.Flush()
}
