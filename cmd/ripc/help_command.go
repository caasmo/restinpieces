package main

import (
	"errors"
)

// ErrUnknownHelpTopic is returned when a help topic is not found.
var ErrUnknownHelpTopic = errors.New("unknown help topic")

// Stored as variables to allow for easy mocking in tests.
var (
	printAppUsageFunc      = printAppUsage
	printGetUsageFunc      = printGetUsage
	printPathsUsageFunc    = printPathsUsage
	printDumpUsageFunc     = printDumpUsage
	printScopesUsageFunc   = printScopesUsage
	printSetUsageFunc      = printSetUsage
	printSaveUsageFunc     = printSaveUsage
	printScaffoldUsageFunc = printScaffoldUsage
	printMigrateUsageFunc  = printMigrateUsage
	printListUsageFunc     = printListUsage
	printDiffUsageFunc     = printDiffUsage
	printRollbackUsageFunc = printRollbackUsage
	printJobUsageFunc      = printJobUsage
	printLogUsageFunc      = printLogUsage
)

// handleHelpCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleHelpCommand(args []string, mainUsage func(), ui UI) error {
	if len(args) == 0 {
		mainUsage()
		return nil
	}

	topic := args[0]
	err := runHelpTopic(topic, ui)
	if err != nil {
		// We only expect ErrUnknownHelpTopic here.
		mainUsage()
		return err
	}
	return nil
}

// runHelpTopic contains the testable core logic for dispatching to the
// correct help printer. It returns an error if the topic is not recognized.
func runHelpTopic(topic string, ui UI) error {
	switch topic {
	case "app":
		printAppUsageFunc(ui.Out)
	case "get":
		printGetUsageFunc(ui.Out)
	case "paths":
		printPathsUsageFunc(ui.Out)
	case "dump":
		printDumpUsageFunc(ui.Out)
	case "scopes":
		printScopesUsageFunc(ui.Out)
	case "set":
		printSetUsageFunc(ui.Out)
	case "save":
		printSaveUsageFunc(ui.Out)
	case "scaffold":
		printScaffoldUsageFunc(ui.Out)
	case "migrate":
		printMigrateUsageFunc(ui.Out)
	case "list":
		printListUsageFunc(ui.Out)
	case "diff":
		printDiffUsageFunc(ui.Out)
	case "rollback":
		printRollbackUsageFunc(ui.Out)
	case "job":
		printJobUsageFunc(ui.Out)
	case "log":
		printLogUsageFunc(ui.Out)
	default:
		return ErrUnknownHelpTopic
	}
	return nil
}
