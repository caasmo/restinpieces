package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/db"
)

func handleJobRm(dbConn db.DbQueueAdmin, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: 'rm' command requires a job ID")
		os.Exit(1)
	}

	jobID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid job ID '%s'. Please provide a number.\n", args[0])
		os.Exit(1)
	}

	err = dbConn.DeleteJob(jobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to delete job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deleted job %d\n", jobID)
}
