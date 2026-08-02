package main

import (
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

// handleJobAddCommand handles the "job add <type>" subcommand. It dispatches
// on the job type token and delegates to the type's handler.
func handleJobAddCommand(dbConn db.DbQueue, args []string, ui UI) error {
	switch args[0] {
	case "backup":
		opts, err := parseJobAddBackupArgs(args[1:])
		if err != nil {
			printJobUsage(ui.Err)
			return err
		}
		return handleJobAddBackupCommand(dbConn, opts, ui)
	default:
		// This case should ideally not be reached if parseJobSubcommand is correct
		printJobUsage(ui.Err)
		return fmt.Errorf("'%s': %w", args[0], ErrUnknownJobType)
	}
}
